package main

import (
	"testing"
	"time"

	"vda5050/common/models"
)

func beepAction(id string, blocking models.BlockingType) models.Action {
	return models.Action{ActionId: id, ActionType: "beep", BlockingType: blocking}
}

func pickAction(id string, blocking models.BlockingType) models.Action {
	return models.Action{ActionId: id, ActionType: "pick", BlockingType: blocking, Retriable: false}
}

// forceElapsed rewinds a queued action's StartedAt far enough into
// the past that completeIfDue treats its fake duration as elapsed,
// without an actual test-slowing sleep.
func forceElapsed(qa *QueuedAction) {
	qa.StartedAt = time.Now().Add(-10 * time.Second)
}

// TestActionEngineHardGate traces [beep(NONE), pick(HARD), beep(NONE)]
// from the plan's "test the matrix" step: the HARD pick must wait for
// the NONE beep ahead of it to finish, and driving must stay stopped
// for as long as pick is anywhere in the queue and non-terminal - not
// just while it's actually running.
func TestActionEngineHardGate(t *testing.T) {
	e := &ActionEngine{}
	e.Enqueue([]models.Action{
		beepAction("b0", models.BlockingTypeNone),
		pickAction("p1", models.BlockingTypeHard),
		beepAction("b2", models.BlockingTypeNone),
	})

	// Pass 1: beep0 starts; pick1 is a HARD gate behind a still-active
	// action, so it waits; beep2 is never even reached.
	e.Advance()
	if got := e.queue[0].Status; got != models.ActionStatusRunning {
		t.Fatalf("beep0 = %s, want RUNNING", got)
	}
	if got := e.queue[1].Status; got != models.ActionStatusWaiting {
		t.Fatalf("pick1 = %s, want WAITING (gated)", got)
	}
	if got := e.queue[2].Status; got != models.ActionStatusWaiting {
		t.Fatalf("beep2 = %s, want WAITING (never reached)", got)
	}
	if e.DrivingAllowed() {
		t.Fatal("DrivingAllowed = true, want false: pick (HARD) is in the queue")
	}

	// Finish beep0; pick1 should now start (nothing else is active).
	forceElapsed(e.queue[0])
	e.Advance()
	if got := e.queue[0].Status; got != models.ActionStatusFinished {
		t.Fatalf("beep0 = %s, want FINISHED", got)
	}
	if got := e.queue[1].Status; got != models.ActionStatusRunning {
		t.Fatalf("pick1 = %s, want RUNNING", got)
	}
	if got := e.queue[2].Status; got != models.ActionStatusWaiting {
		t.Fatalf("beep2 = %s, want still WAITING", got)
	}
	if e.DrivingAllowed() {
		t.Fatal("DrivingAllowed = true, want false: pick (HARD) is RUNNING")
	}

	// Finish pick1 (Retriable: false, so it always lands on a terminal
	// status - FINISHED or FAILED - never RETRIABLE). beep2 can now
	// start, and driving may resume.
	forceElapsed(e.queue[1])
	e.Advance()
	if got := e.queue[0].Status; !isTerminal(got) {
		t.Fatalf("pick1 = %s, want a terminal status", got)
	}
	if got := e.queue[1].Status; got != models.ActionStatusRunning {
		t.Fatalf("beep2 = %s, want RUNNING", got)
	}
	if !e.DrivingAllowed() {
		t.Fatal("DrivingAllowed = false, want true: no SOFT/HARD action remains non-terminal")
	}
}

// TestActionEngineSoftRunsInParallel traces [beep(SOFT), pick(SOFT)]:
// SOFT allows other actions, so both start in the very same Advance()
// pass - no gating between them, unlike the HARD case above. Driving
// still stops, though: SOFT permits parallelism, not driving.
func TestActionEngineSoftRunsInParallel(t *testing.T) {
	e := &ActionEngine{}
	e.Enqueue([]models.Action{
		beepAction("b0", models.BlockingTypeSoft),
		pickAction("p1", models.BlockingTypeSoft),
	})

	e.Advance()
	if got := e.queue[0].Status; got != models.ActionStatusRunning {
		t.Fatalf("beep0 = %s, want RUNNING", got)
	}
	if got := e.queue[1].Status; got != models.ActionStatusRunning {
		t.Fatalf("pick1 = %s, want RUNNING (SOFT runs alongside SOFT)", got)
	}
	if e.DrivingAllowed() {
		t.Fatal("DrivingAllowed = true, want false: SOFT actions are present")
	}
}
