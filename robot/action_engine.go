package main

import (
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"time"

	vdaerrors "vda5050/common/errors"
	"vda5050/common/models"
)

// actionDurations are the fake action durations for this session's
// stand-ins (pick, drop, beep) - no real hardware to wait on yet.
var actionDurations = map[string]time.Duration{
	"pick": 3 * time.Second,
	"drop": 3 * time.Second,
	"beep": 1 * time.Second,
}

// pickFailRate makes pick fail 30% of the time, to exercise the
// RETRIABLE path.
const pickFailRate = 0.3

type QueuedAction struct {
	Action    models.Action
	Status    models.ActionStatus
	Result    string
	StartedAt time.Time // when it entered RUNNING, for the fake timed actions
}

type ActionEngine struct {
	mu    sync.Mutex
	queue []*QueuedAction
}

// Enqueue appends a node's (or edge's) actions to the back of the
// queue, in array order, each starting WAITING.
func (e *ActionEngine) Enqueue(actions []models.Action) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, action := range actions {
		e.queue = append(e.queue, &QueuedAction{
			Action:    action,
			Status:    models.ActionStatusWaiting,
			Result:    "",
			StartedAt: time.Time{},
		})
	}
}

// isTerminal reports whether an action is done and no longer occupies
// a slot in the driving/parallelism decisions.
func isTerminal(status models.ActionStatus) bool {
	return status == models.ActionStatusFinished || status == models.ActionStatusFailed
}

// isActive reports whether an action currently occupies a slot -
// already started and not yet terminal. RETRIABLE counts as active:
// it isn't removed from the queue, it just sits until an external
// retry/skipRetry resolves it.
func isActive(status models.ActionStatus) bool {
	switch status {
	case models.ActionStatusInitializing, models.ActionStatusRunning,
		models.ActionStatusPaused, models.ActionStatusRetriable:
		return true
	default:
		return false
	}
}

// start transitions a WAITING action to RUNNING and begins its timer.
// INITIALIZING is skipped - the spec allows omitting it when the
// robot transitions instantly, which our fake actions do.
func (e *ActionEngine) start(qa *QueuedAction) {
	qa.Status = models.ActionStatusRunning
	qa.StartedAt = time.Now()
}

// completeIfDue resolves a RUNNING fake action once its duration has
// elapsed: pick may fail (RETRIABLE if retriable, else FAILED
// directly), everything else FINISHES. Unknown action types are left
// running forever - nothing else in this session completes them.
func (e *ActionEngine) completeIfDue(qa *QueuedAction) {
	duration, known := actionDurations[qa.Action.ActionType]
	if !known || time.Since(qa.StartedAt) < duration {
		return
	}

	if qa.Action.ActionType == "pick" && rand.Float64() < pickFailRate {
		if qa.Action.Retriable {
			qa.Status = models.ActionStatusRetriable
		} else {
			qa.Status = models.ActionStatusFailed
		}
		return
	}

	qa.Status = models.ActionStatusFinished
}

// Advance runs one pass of Figure 11: drop terminal actions, then
// walk the queue front-to-back starting/continuing actions until it
// hits a SINGLE/HARD action that isn't ready yet (or is currently the
// sole active one) - then stop. Called on a tick, same coalescing
// style as StatePublisher/Simulator.
func (e *ActionEngine) Advance() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.queue = slices.DeleteFunc(e.queue, func(qa *QueuedAction) bool {
		return isTerminal(qa.Status)
	})

	anyActive := false
	for _, qa := range e.queue {
		if qa.Status == models.ActionStatusRunning {
			e.completeIfDue(qa)
		}

		switch {
		case isActive(qa.Status):
			anyActive = true
			if qa.Action.BlockingType == models.BlockingTypeSingle || qa.Action.BlockingType == models.BlockingTypeHard {
				return
			}
		case qa.Status == models.ActionStatusWaiting:
			switch qa.Action.BlockingType {
			case models.BlockingTypeNone, models.BlockingTypeSoft:
				e.start(qa)
				anyActive = true
			default: // SINGLE, HARD
				if !anyActive {
					e.start(qa)
				}
				return
			}
		}
	}
}

// DrivingAllowed scans the WHOLE queue (not just the active head) for
// any non-terminal SOFT or HARD action. This is the asymmetry the
// plan calls out: driving is a whole-queue question, parallelism is a
// head-of-queue question.
func (e *ActionEngine) DrivingAllowed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, qa := range e.queue {
		if isTerminal(qa.Status) {
			continue
		}
		if qa.Action.BlockingType == models.BlockingTypeSoft ||
			qa.Action.BlockingType == models.BlockingTypeHard {
			return false
		}
	}
	return true
}

// ActionStates snapshots the queue as []models.ActionState, for
// buildState to merge with horizon actions (always WAITING). Only
// covers actions already Enqueue'd (i.e. from reached nodes/edges) -
// horizon actions aren't in this queue yet, so merging them in is the
// caller's job.
func (e *ActionEngine) ActionStates() []models.ActionState {
	e.mu.Lock()
	defer e.mu.Unlock()

	states := make([]models.ActionState, len(e.queue))
	for i, qa := range e.queue {
		states[i] = models.ActionState{
			ActionId:         qa.Action.ActionId,
			ActionType:       qa.Action.ActionType,
			ActionDescriptor: qa.Action.ActionDescriptor,
			ActionStatus:     qa.Status,
			ActionResult:     qa.Result,
		}
	}
	return states
}

// Retry moves a RETRIABLE action back to RUNNING (restarts its timer).
func (e *ActionEngine) Retry(actionId string) *vdaerrors.VDAError {
	e.mu.Lock()
	defer e.mu.Unlock()

	ref := vdaerrors.ErrorReference{ReferenceKey: "actionId", ReferenceValue: actionId}

	for _, qa := range e.queue {
		if qa.Action.ActionId != actionId {
			continue
		}
		if qa.Status != models.ActionStatusRetriable {
			return vdaerrors.New(
				vdaerrors.ErrorTypeInvalidInstantAction,
				fmt.Sprintf("action %s is not RETRIABLE (status=%s)", actionId, qa.Status),
				ref,
			)
		}
		qa.Status = models.ActionStatusRunning
		qa.StartedAt = time.Now()
		return nil
	}

	return vdaerrors.New(
		vdaerrors.ErrorTypeInvalidInstantAction,
		fmt.Sprintf("action %s not found", actionId),
		ref,
	)
}

// SkipRetry forces a RETRIABLE action straight to FAILED instead of
// giving it another attempt.
func (e *ActionEngine) SkipRetry(actionId string) *vdaerrors.VDAError {
	e.mu.Lock()
	defer e.mu.Unlock()

	ref := vdaerrors.ErrorReference{ReferenceKey: "actionId", ReferenceValue: actionId}

	for _, qa := range e.queue {
		if qa.Action.ActionId != actionId {
			continue
		}
		if qa.Status != models.ActionStatusRetriable {
			return vdaerrors.New(
				vdaerrors.ErrorTypeInvalidInstantAction,
				fmt.Sprintf("action %s is not RETRIABLE (status=%s)", actionId, qa.Status),
				ref,
			)
		}
		qa.Status = models.ActionStatusFailed
		return nil
	}

	return vdaerrors.New(vdaerrors.ErrorTypeInvalidInstantAction, fmt.Sprintf("action %s not found", actionId), ref)
}

// CancelAll fails every non-terminal action, for cancelOrder (spec
// §6.1.3). Simplification: factsheet's per-action cancelAllowed isn't
// modeled yet (Session 7), so every action is treated as cancellable
// - a running action is FAILED immediately rather than left RUNNING
// to its real outcome.
func (e *ActionEngine) CancelAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, qa := range e.queue {
		if !isTerminal(qa.Status) {
			qa.Status = models.ActionStatusFailed
		}
	}
}
