# VDA 5050 from Scratch — A 10-Hour Build-It-Yourself Curriculum

Based on **VDA 5050 v3.0.0** (the PDF in this project).

Language for the code-along: **Go** (`eclipse/paho.mqtt.golang` + the standard library `encoding/json` + a local `mosquitto` broker). Nothing in the plan is Go-specific — if you'd rather do Python, TypeScript, or C#, the sessions and milestones are identical, only the syntax changes.

---

## 0. Before we start: the five ideas everything hangs off

If you remember nothing else, remember these. Every weird detail in the spec is a consequence of one of them.

**1. VDA 5050 is a wire, not a brain.**
It defines _what messages travel between one fleet manager and one robot_. It does not define how to plan traffic, how to be safe, how to navigate, or how to be secure. Those are deliberately out of scope.

**2. The floor is a graph.**
Not coordinates, not "drive to (12.4, 3.1)". Named **nodes** (places you can stand) and **edges** (permitted moves between them). Everything — traffic control, orders, zones — is built on this.

**3. Wi-Fi is unreliable, therefore promises are immutable.**
The fleet manager can never know whether a message arrived. So once it _releases_ a piece of route to the robot, it can never take it back. This single physical fact produces the base/horizon split, order stitching, sequence IDs, and the admission that "cancel" is best-effort.

**4. The robot reports a full snapshot, not deltas.**
One fat `state` message, sent on events and at least every 30 s. If you miss one, the next one repairs you. No diff-application, no replay log.

**5. Everything is extensible, and the robot describes itself.**
Enums can be extended. Actions can be vendor-specific. So the robot publishes a **factsheet** — "here's what I can do, here are my limits" — and the fleet manager configures itself from that.

---

## Setup (do this before Session 1, ~15 min, not counted in the 10 hours)

```bash
# broker
sudo apt install mosquitto mosquitto-clients      # or: brew install mosquitto
mosquitto -v -p 1883

# go
go mod init vda5050
go get github.com/eclipse/paho.mqtt.golang
```

Repo layout you'll grow into over the 10 hours:

```
vda5050/
  common/
    header.go        # S2 — the 5 fields on every message
    topics.go         # S2 — topic construction
    models/          # S3, S5, S6, S7, S8 — message schemas (structs + JSON tags)
  robot/
    main.go          # the robot side
    order_state.go   # S4 — base/horizon + acceptance
    action_engine.go # S6 — blocking types
  fleet/
    main.go          # the fleet-control side
    dispatcher.go    # S4 — order splitting & extension
  sim/
    kinematics.go    # S3 — fake driving
  viewer/
    main.go          # S2 — MQTT<->WebSocket bridge (net/http, paho, gorilla/websocket)
    static/
      index.html     # S2 onward — canvas map + control panel, grows every session
```

### The control panel — introduced now, grown every session

Starting in Session 2 you'll build `viewer/`, a small Go service that sits between the
broker and your browser and never leaves. It's not a Session-9 extra — it's the window
you watch every experiment through from here on:

```
Browser (canvas + buttons, plain JS, no framework)
    ▲│ WebSocket
    │▼
viewer/main.go  (subscribes to broker topics, forwards as WS frames;
                 turns button clicks into published MQTT messages)
    ▲│ MQTT
    │▼
mosquitto broker
```

The browser never speaks MQTT directly — the Go bridge is the only MQTT client on
that side, which is exactly the client/server split you already know from `robot/`
and `fleet/`. Each session adds one thing to draw and one or two buttons to press,
listed under that session's own Code milestone below.

The **official JSON schemas** live at `github.com/vda5050/vda5050`. Don't read them yet — you'll appreciate them far more after Session 4.

---

## The 10 sessions

| #   | Topic                                         | Time   |
| --- | --------------------------------------------- | ------ |
| 1   | The problem VDA 5050 solves                   | 45 min |
| 2   | Transport: MQTT, JSON, topics, connection     | 60 min |
| 3   | The order: graph, nodes, edges                | 75 min |
| 4   | Base & horizon — the heart of the protocol    | 75 min |
| 5   | State: how the robot talks back               | 60 min |
| 6   | Actions                                       | 75 min |
| 7   | Factsheet: self-description                   | 45 min |
| 8   | Maps, zones, requests & responses             | 60 min |
| 9   | Visualization, planned paths, errors          | 45 min |
| 10  | Strengths, weaknesses, and a 2-robot capstone | 60 min |

**Total: 600 minutes.**

---

# Session 1 — The problem (45 min)

### Explain it to a kid

Imagine a school gym with 30 kids, and one teacher on a balcony who can see everything. The teacher shouts: "Ana, walk to the blue mat. Ben, wait — Ana's crossing." Nobody bumps into anybody, because one person with the whole picture is giving instructions, and the kids just do what they're told and shout back what they're doing.

Now the awkward part. The kids come from three different countries. One only understands Spanish, one only Japanese, one only Finnish. The teacher needs three interpreters, and every time a new kid arrives from a new country, you hire another interpreter.

**VDA 5050 is the decision that all the kids will learn one shared playground language.** Not a _full_ language — just enough to say "go here", "I'm here", "I lifted the box", "my battery is low".

### Now the grown-up version

- A **mobile robot** (the spec's word — covers AGVs, AMRs, forklifts, tuggers) moves material around a factory or warehouse.
- A **fleet control system** (also called a fleet manager, or the master) decides who goes where and when, so robots don't deadlock or collide.
- Before VDA 5050, every robot vendor had a proprietary interface. An integrator running robots from 3 vendors wrote and maintained 3 drivers. Buying a 4th vendor meant a new integration project. This is the classic **N×M problem**: N fleet managers × M robot vendors = N×M integrations. A standard turns it into **N+M**.
  VDA 5050 was written jointly by the **VDA** (German automotive industry association) and **VDMA** (mechanical engineering industry association), with development managed by **IFL at KIT**. That origin matters: it was pulled into existence by _car factories_ who were tired of vendor lock-in, not by robot makers.

### The stated objectives (spec §2)

1. Reduce complexity when connecting mobile robots to a fleet control system.
2. Enable coordinated operation of **heterogeneous** fleets — different manufacturers, same floor.
3. Be **generic and domain-independent** — works for line-guided robots and freely navigating ones, any size, any load handling, any autonomy level.

### What it explicitly does NOT cover (spec §2) — and _why_ each exclusion is deliberate

| Out of scope                         | Why                                                                                                                                                                                  |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Safety** requirements              | Safety is regulated elsewhere (ISO 3691-4, ISO 13849, certified safety hardware). A JSON message over Wi-Fi can never be a safety function. Pretending otherwise would be dangerous. |
| **Traffic management logic**         | Routing, prioritization, deadlock resolution — this is the hardest problem and the main thing fleet-manager vendors compete on. Standardizing it would kill the market.              |
| **Other interfaces**                 | Peripherals, conveyors, doors, WMS/ERP. Keeping the scope to one wire is what makes the standard implementable.                                                                      |
| **Cybersecurity**                    | "Configure your broker." A real weakness — we'll come back to it in Session 10.                                                                                                      |
| **Project/commissioning procedures** | Organizational, not technical.                                                                                                                                                       |
| **Responsibility allocation**        | Legal, not technical.                                                                                                                                                                |

### Two definitions you'll need constantly (spec §3.6, §3.7)

- **Line-guided robot**: follows _predefined trajectories_ — either sent by fleet control inside the order, or stored on the robot, or implied as the straight line between two nodes.
- **Freely navigating robot**: plans its _own_ trajectories. But — and this is the key subtlety — if fleet control sends a trajectory in the order, it must follow it.
  Almost every optional feature in the spec exists to accommodate one or the other of these.

### Code milestone

None. Draw this on paper instead:

```
        ┌──────────────────────┐
        │   Fleet Control      │
        │  (routes, traffic)   │
        └───────┬──────┬───────┘
        publishes│      │subscribes
                 ▼      ▲
        ┌────────────────────────┐
        │      MQTT Broker       │
        └────────────────────────┘
                 ▲      │
        subscribes│      │publishes
        ┌────────┴──────▼───────┐
        │     Mobile Robot      │
        │  (drives, reports)    │
        └───────────────────────┘
```

Then get `mosquitto` running and prove you can `mosquitto_pub` / `mosquitto_sub` a "hello" between two terminals.

### Check yourself

- Explain to somebody why a _warehouse operator_ cares about VDA 5050 more than a _robot manufacturer_ does.
- Two "VDA 5050 compliant" products from different vendors are plugged together. Name three ways they might still fail to work well together. (Answer arrives in Session 10.)

---

# Session 2 — Transport: MQTT, JSON, topics, connection (60 min)

### Explain it to a kid

Instead of everybody phoning everybody, there's one big corkboard in the hallway. It has labeled sections: "Orders for Robot 7", "How Robot 7 is doing". You pin a note to a section; anyone who cares about that section is told immediately. You never need to know who's reading, or whether they're even in the building.

That's **publish/subscribe**, and MQTT is a very small, very old, very reliable version of it.

### Why MQTT and not HTTP/REST? (spec §4)

Every reason traces back to "the robot is moving and on Wi-Fi":

- **Robots have no stable address.** With REST, the fleet manager would need to know each robot's IP and reach it. With MQTT, _both sides dial out_ to one known broker. NAT, roaming, DHCP — all irrelevant.
- **One-to-many for free.** The robot's position is wanted by the fleet manager, the visualization UI, the analytics logger. One publish, three subscribers, robot doesn't know or care.
- **Tiny.** MQTT works on constrained embedded controllers. (Minimum required version: **MQTT 3.1.1**.)
- **Death detection is built in** (the _last will_, below).
- Communication is explicitly **expected to be wireless, with connection failures and lost messages** (§4). The transport was chosen to survive that, not to hide it.

### Why JSON?

Human-readable (you can debug with `mosquitto_sub` and your eyes), **validatable against schemas**, and **extensible** — a v3.1 robot can add an optional field and a v3.0 parser just ignores it. That last property is why the spec's version scheme works at all.

### Topic structure (spec §4.2)

```
interfaceName / majorVersion / manufacturer / serialNumber / topic

vda5050 / v3 / KIT / 0001 / order
```

Unpack _why_ each level is there:

- `interfaceName` — the broker may carry other traffic. Namespace it.
- `majorVersion` (with the `v` prefix) — **this is the clever one.** Major versions are breaking. Putting the version in the _topic_ means a v2 robot and a v3 robot can coexist on the same broker during a migration, and a v3 fleet manager simply doesn't subscribe to `v2/...`. No handshake, no negotiation.
- `manufacturer` + `serialNumber` — natural addressing. And it gives fleet control wildcard power: `vda5050/v3/+/+/state` subscribes to every robot in the building in one line.
- `topic` — the message kind.
  Constraints: `/` is the hierarchy separator so it can't appear in any field; avoid the wildcards `+` and `#` and broker-reserved `$`. `serialNumber` is limited to `A-Z a-z 0-9 _ . : -`.

Note the spec's honesty: the _structure_ is a suggestion (cloud brokers like AWS IoT impose their own), but **the topic names are mandatory**.

### The eight topics (spec §4.3)

| Topic            | Direction         | Mandatory | Purpose                                |
| ---------------- | ----------------- | --------- | -------------------------------------- |
| `order`          | FC → robot        | yes       | Where to drive, what to do             |
| `instantActions` | FC → robot        | yes       | Do this _now_, regardless of the order |
| `state`          | robot → FC        | yes       | Everything about the robot             |
| `connection`     | robot/broker → FC | yes       | Am I alive?                            |
| `factsheet`      | robot → FC        | yes       | What am I capable of?                  |
| `visualization`  | robot → UI        | optional  | High-frequency position + path         |
| `zoneSet`        | FC → robot        | optional  | Rules painted on the floor             |
| `responses`      | FC → robot        | optional  | Answers to the robot's requests        |

Notice the shape: **four mandatory, four optional.** The mandatory four are the minimum viable conversation: _here's your job / here's how it's going / are you alive / who are you_.

### QoS — and why it's not uniform (spec §4.1)

**QoS 0 (best effort)** for `order`, `instantActions`, `state`, `factsheet`, `zoneSet`, `responses`, `visualization`.
**QoS 1 (at least once)** for `connection`.

Why is this not insane? Because of Idea #4: `state` is a **full snapshot** sent at least every 30 s. Losing one costs you at most 30 s of staleness and the system self-heals. Higher QoS would mean broker-side queues, retries, and per-message acknowledgement overhead for data that's about to be superseded anyway.

`connection` is the exception because a _death notice_ is not superseded by anything. If you miss "robot 7 fell off the network", you never learn it. Hence QoS 1 — and **retained**, so a fleet manager that starts up later immediately learns the current connection state of every robot.

### Last will — the elegant bit (spec §6.5)

The robot tells the broker at connect time: _"if I stop breathing, publish this on my behalf."_

```
On connect:
  1. Set last will → topic .../connection, connectionState = 'CONNECTION_BROKEN'
  2. Publish        → topic .../connection, connectionState = 'ONLINE'

On graceful shutdown:
  1. Publish        → connectionState = 'OFFLINE'
  2. MQTT DISCONNECT
```

Four connection states: `ONLINE`, `OFFLINE` (planned), `HIBERNATING` (low-power, still connected, not sending state), `CONNECTION_BROKEN` (the last will fired).

The spec adds a warning worth internalizing: **do not use the `connection` topic to check robot health.** It's a _protocol-level_ liveness check only. A robot can be perfectly ONLINE and completely stuck. Health lives in `state`.

Also note: if the robot loses the broker, it **keeps its order and finishes it up to the last released node.** It doesn't panic-stop. Idea #3 in action.

### The header — on every single message (spec §7.2)

```json
{
  "headerId": 42,
  "timestamp": "2026-07-30T11:40:03.123Z",
  "version": "3.0.0",
  "manufacturer": "KIT",
  "serialNumber": "0001"
}
```

Two details with real reasoning behind them:

- `headerId` is **per topic** and incremented by 1 for each message **sent** — explicitly _not_ per message received. So the receiver can detect gaps ("I saw state 41 then 43, I lost one") without any reliable-delivery machinery. It's a sequence counter for a lossy channel.
- The header is **not a nested JSON object** — the five fields sit flat at the top level of every message. Slightly ugly, but it keeps parsers simple and messages shallow.

### Code milestone

1. `common/header.go` — a header factory with a per-topic counter (a struct with a mutex-guarded `int`).
2. `common/topics.go` — `TopicFor(manufacturer, serial, topic string) string`.
3. `robot/main.go` — connects, sets last will (`CONNECTION_BROKEN`, retained, QoS 1), publishes `ONLINE`, subscribes to `order` and `instantActions`.
4. `fleet/main.go` — subscribes `vda5050/v3/+/+/state` and `vda5050/v3/+/+/connection`, prints what it sees.
5. `viewer/main.go` — the bridge is born here: subscribe `vda5050/v3/+/+/connection`,
   forward each message over a WebSocket. `viewer/static/index.html` — one row per
   robot showing `manufacturer/serialNumber` and its connection state, colour-coded.
6. **Kill the running `robot/main.go` binary with `kill -9`.** Watch `CONNECTION_BROKEN` appear at the fleet manager **and turn the row red in the browser.** That's the moment the transport design clicks — twice.

### Check yourself

- Why is the major version in the topic path rather than only in the message body (where it also appears)?
- Your fleet manager restarts. Why does it immediately know the connection state of all 40 robots, but _not_ their positions?

---

# Session 3 — The order: graph, nodes, edges (75 min)

### Explain it to a kid

The factory floor is a board game. **Nodes** are the squares you're allowed to stand on. **Edges** are the arrows showing which squares you may move between. An order is a strip of paper: _square A, arrow 1, square B, arrow 2, square C._ Always starts on a square, always ends on a square, always alternating.

### Why a graph? (spec §6.1)

This is a design decision, not a technicality, and there are three reasons:

1. **Traffic control needs countable resources.** You can't reserve "the area around (12.4, 3.1)". You _can_ reserve node B and edge 2. Discrete graph elements are lockable, queueable, schedulable.
2. **It unifies both robot types.** For a line-guided robot, the edge _is_ the physical line. For a freely navigating robot, the edge is a permission to get from A to B however it likes. Same message, two interpretations.
3. **It keeps route authority centralized.** Fleet control owns the graph. The robot never chooses which way to go across the facility.
   And critically (§6.1, Figure 2): **fleet control sends only the sub-graph for this journey**, never the whole facility map. The robot has no idea the building has 4,000 nodes. Smaller messages, and — more importantly — the robot cannot second-guess the route.

### The order message (spec §7.3)

```json
{
  "headerId": 1, "timestamp": "...", "version": "3.0.0",
  "manufacturer": "KIT", "serialNumber": "0001",
  "orderId": "transport-4711",
  "orderUpdateId": 0,
  "orderDescription": "Pallet from receiving to rack 12",
  "nodes": [ ... ],
  "edges": [ ... ]
}
```

`orderDescription` carries a health warning in the spec: _human-readable, visualization only, **shall not** be used for any logical process._ You'll see this warning on every `*Descriptor` field in the spec. It exists because integrators inevitably start parsing free-text fields, and then nothing is interoperable any more.

### `nodeId` vs `sequenceId` — dwell here

This is the single most common source of confusion. Both exist, and they answer different questions.

- `nodeId` = **which place**. "The charging station." Reused every time you visit it.
- `sequenceId` = **which step of this journey**. Shared across _both_ nodes and edges, monotonically increasing, defining traversal order.
  Why both? Because a route can visit the same place twice:

```
sequenceId:  0     1     2     3     4     5     6
element:    n:A   e:1   n:B   e:2   n:C   e:3   n:B
```

Node B appears at sequence 2 and at sequence 6. `nodeId` alone can't tell them apart. Convention: **nodes get even `sequenceId`s, edges get odd**, which falls naturally out of the alternation and makes off-by-one bugs obvious.

Second reason: `sequenceId` is the anchor for order updates (Session 4). Once assigned to a released node, **it never changes**.

### Node structure

```json
{
  "nodeId": "n_B",
  "sequenceId": 2,
  "nodeDescriptor": "Aisle 3 entry",
  "released": true,
  "nodePosition": {
    "x": 12.4,
    "y": 3.1,
    "theta": 1.57,
    "mapId": "floor1",
    "allowedDeviationXY": { "a": 0.3, "b": 0.1, "theta": 0.0 },
    "allowedDeviationTheta": 0.05
  },
  "actions": []
}
```

- `nodePosition` is **optional** — line-guided robots may not need it. The graph is topological first, geometric second.
- `released` — the base/horizon flag. Session 4.

### `allowedDeviationXY` — an ellipse, and why (spec §6.6.2, Figure 20)

Not a radius. An **ellipse**: semi-major axis `a`, semi-minor axis `b`, rotation `theta`, centred on the node.

Why an ellipse? Because tolerance _along_ the direction of travel is a different engineering question from tolerance _across_ it. In a narrow aisle you can afford ±40 cm along the path and only ±5 cm sideways. A circle would force you to the smaller of the two everywhere.

What it actually buys you: **corner cutting.** Inside the ellipse the robot may leave its predefined trajectory and take a smooth arc instead of driving to the exact point and pivoting. The rule (§6.6.2): when it _leaves_ the ellipse it must be back on the predefined trajectory of the next edge.

`a = b = 0.0` means "as precise as technically possible" — and if the robot supports the field but fleet control didn't send it, the robot **must assume 0.0**. Fail-strict, not fail-loose. Same for `allowedDeviationTheta`.

### Edge structure (the interesting fields)

```json
{
  "edgeId": "e_1", "sequenceId": 1,
  "startNodeId": "n_A", "endNodeId": "n_B",
  "released": true,
  "maximumSpeed": 1.2,
  "maximumMobileRobotHeight": 2.1,
  "orientation": 0.0,
  "orientationType": "TANGENTIAL",
  "rotationAllowed": true,
  "trajectory": { "degree": 3, "knotVector": [...], "controlPoints": [...] },
  "corridor": { ... },
  "actions": []
}
```

- **`orientation` + `orientationType`** — `TANGENTIAL` means relative to the path (0 = forwards, π = backwards, which is how you tell a forklift to reverse into a rack). `GLOBAL` means relative to the map. Plus `reachOrientationBeforeEntering`: rotate _before_ the edge or _on_ it. If it's `true` and the robot can't, **the order must be rejected** — not silently approximated.
- **`trajectory` as a NURBS curve** (degree, knot vector, weighted control points). Why NURBS rather than a polyline? A few control points describe an exactly-reproducible smooth curve, including circular arcs, in very few bytes. Polylines either look jerky or get huge. This is how fleet control dictates a precise path through a tight spot while leaving the robot free elsewhere.
- **`corridor`** — a left/right width around the trajectory inside which the robot may deviate to dodge an obstacle. We'll do this properly in Session 8, because it's tied to the request/response mechanism.
- **Actions on edges**, not just nodes — e.g. "keep the beacon on for the length of this aisle."

### Structural rules to encode as validators

- Order alternates node, edge, node, ... and **starts and ends with a node**.
- `sequenceId` strictly increasing; nodes even, edges odd.
- Every edge's `startNodeId`/`endNodeId` match its neighbours in the sequence.
- The **first node** must be where the robot already is — standing on it, or within its deviation range (§6.6.1). Consequence: the first node is **not** reported in `nodeStates`.
- An edge may only be `released` if both its nodes are.
- After an unreleased element, nothing released may follow.

### Code milestone

1. `common/models/order.go` — structs with `json:"..."` tags for `Order`, `Node`, `Edge`, `NodePosition`, `Trajectory`.
2. A `Validate(order Order) error` function implementing every structural rule above, returning a VDA 5050 error type on failure (`VALIDATION_FAILURE`, `INVALID_ORDER`).
3. `sim/kinematics.go` — dumb simulator: linear interpolation from node to node at `maximumSpeed`, 10 Hz tick (`time.Ticker`), deviation-ellipse check to decide traversal.
4. Hand-write a 5-node order in a JSON file, publish it, watch the robot "drive" it in the terminal.
5. **Viewer:** draw the graph. The bridge forwards `state.mobileRobotPosition`; the
   canvas plots nodes as dots (labelled with `nodeId`), edges as lines between them,
   and the robot as a moving marker. Add a text box + "Send Order" button that POSTs
   your hand-written JSON order to the bridge, which publishes it. Now you trigger
   Step 4 from the browser instead of a second terminal.

### Check yourself

- Write an order where the robot goes A → B → A → C. How many entries in `nodes`? What are their `nodeId`s and `sequenceId`s?
- Why is `nodePosition` optional but `nodeId` mandatory?
- A node has `allowedDeviationXY` of `a=0.5, b=0.05, theta=0`. Sketch where the robot is allowed to be. Now the same node on a path running north-south — what should `theta` be?

---

# Session 4 — Base & horizon: the heart of the protocol (75 min)

**This is the session that matters most.** If you understand base/horizon and stitching, the rest of VDA 5050 is paperwork.

### Explain it to a kid

The teacher says: _"Walk to the blue mat, then the red mat. After that I'm **thinking** you'll go to the window — but don't count on it, I'll tell you when I'm sure."_

- Blue mat, red mat = **the base**. Promises. Already yours.
- The window = **the horizon**. A plan. Not permission.
- The red mat (last promised spot) = **the decision point**. If nothing new arrives, you stop there and wait.

### Why split the route at all? (spec §6.1.2)

Two reasons, both practical:

1. **Traffic management.** Fleet control wants to hand out only the floor it's certain about. If two robots are converging on one corridor, it releases the corridor to one and holds the other at its decision point. The base _is_ the reservation.
2. **Resource-constrained robots.** A full transport order might be 200 nodes. A small controller can't hold that. Split it into sub-orders sharing an `orderId`.

### Why the base can NEVER be changed — the load-bearing insight

Straight from §6.1.2:

> _Since MQTT is an asynchronous protocol and transmission via wireless networks is not reliable, the base cannot be changed. The fleet control shall therefore assume that the base has already been executed by the mobile robot._

Sit with that. Fleet control publishes "you may drive to node B" and then... has no idea what happened. Did the robot get it? Is it already there? Is it halfway? By the time any answer comes back, it's stale.

So the protocol makes the only safe assumption: **once released, treat it as done.** You cannot un-release. You cannot edit. You can only _append_.

Everything else follows:

- Order updates only extend, never modify → **stitching**.
- `sequenceId` of a released node never changes → the two sides can agree on history without a transaction log.
- The spec itself calls `cancelOrder` **unreliable** and designs around that.
- Reservations must be conservative, because releasing is irrevocable.
  This is a distributed-systems design under an unreliable channel, and it's genuinely well done.

### The mechanics

`released: true` = base. `released: false` = horizon. Rules:

- An edge is released only if both its nodes are.
- After an unreleased edge, no released node or edge may follow.
- **An order with no horizon at all is valid.**
- The horizon may be changed, shortened, or deleted entirely with any update — the base extension doesn't even have to follow the previous horizon. It was only ever a suggestion.
- The robot **stops at the decision point** if the base isn't extended. Fleet control _should_ extend before arrival for fluid motion.

### Order updates and stitching (spec §6.1.2, Figures 4–7)

Initial order:

```
orderId: "1234", orderUpdateId: 0
nodes: [ f(released), d(released), g(released), b(horizon), h(horizon) ]
edges: [ e1(released), e3(released), e8(horizon), e9(horizon) ]
```

Decision point = **g**. Update:

```
orderId: "1234", orderUpdateId: 1
nodes: [ g(released), b(released), h(released), i(horizon) ]
edges: [ e8(released), e9(released), e10(horizon) ]
```

Three things to notice, each with a reason:

1. **`orderId` stays, `orderUpdateId` increments.** Same journey, next chapter. `orderUpdateId` starts at 0 for a new order and must be unique per `orderId`.
2. **The update starts with `g` — the previous decision point.** This is the **stitching node**. Why: (a) it proves the new base is _reachable_ from what the robot already knows; (b) it's a free integrity check — the robot compares `nodeId` + `sequenceId` and knows whether it missed an update. On a lossy channel, that's worth more than an ack.
3. **`f` and `d` are NOT resent.** The base can't change, so retransmitting it is invalid, not merely wasteful.
   And the rule people get wrong: **the stitching node's content must be resent identically** — same actions, same deviations, same everything. It's a re-declaration of an immutable fact, not a new instruction.

### The awkward special case (Figure 7)

You want the robot to do something on the node it's already parked on, and you only decided that after it got there. You can't modify the existing node — it's base.

The workaround: resend the decision node _with all its old metadata_ (including actions already `FINISHED`/`RUNNING`, which are **not** re-executed), then append a **new node** at the same physical position carrying the new actions. Its `sequenceId` is the decision node's **+2**.

It's clunky. It's clunky _because_ immutability is non-negotiable, and this is the cheapest legal way to fake mutation. Understanding why it's ugly means you've understood the constraint.

### Order acceptance — the decision tree (spec §6.1.2, Figure 8)

Eleven checks the robot runs on every incoming order. Implement it as one explicit function:

1. Is the order **valid**? (JSON, types, structure) → else `VALIDATION_FAILURE` / `INVALID_ORDER`
2. Is it a **new order** or an **update**? (`orderId` different or same?)
3. If new: is the robot **idle and not awaiting an update**? Careful — _having a horizon counts as awaiting an update._ → else `OTHER_ORDER_ACTIVE`
4. If new: is `orderUpdateId == 0`?
5. If new: is the **first node close enough** — standing on it or within its deviation range? → else `START_NODE_OUT_OF_RANGE`
6. If update: is it **deprecated**? (`orderUpdateId` ≤ current) → `OUTDATED_ORDER_UPDATE`
7. Does it follow a **cancelOrder**? Then reject → `ORDER_UPDATE_FOLLOWING_CANCEL`
8. Is `orderUpdateId` **equal** to the current one? → `SAME_ORDER_UPDATE_ID`
9. Is it a **valid continuation of a still-running order**? (first node == previous decision point)
10. Is it a **valid continuation of a completed order**? (same test, robot now idle)
11. Accept: populate/append `nodeStates`, `edgeStates`, `actionStates`.
    Checks 9 and 10 are the same test in two different robot states — worth noticing, because it means "am I still driving?" doesn't change the stitching contract at all.

### Cancellation (spec §6.1.3, Figure 9)

`cancelOrder` is an **instant action**, optionally carrying an `orderId`. On receipt:

- Stop as soon as possible. Line-guided: next feasible node. Freely navigating: **as soon as possible, not merely at the next node.**
- Scheduled actions → cancelled, reported `FAILED`.
- Running actions → cancelled if possible, reported `FAILED`. If **not** cancellable (`cancelAllowed: false`), report `RUNNING` until done, then the real outcome.
- `orderId`, `orderUpdateId`, `lastNodeId`, `lastNodeSequenceId` are **unchanged**.
- `nodeStates` and `edgeStates` → empty. Pending requests → removed.
- No further updates to a cancelled order may be sent or accepted.
  Why do FAILED and not CANCELLED? Because the spec keeps the `actionStatus` enum small and treats "didn't complete" as one outcome. Fewer states, fewer disagreements.

### Code milestone

1. `robot/order_state.go` — the 11-check acceptance function, returning `(accepted bool, err *VDAError)`. Table-driven tests (`go test`) for each branch.
2. Base/horizon storage; append-only base; horizon replaceable.
3. `fleet/dispatcher.go` — takes a full route, releases the first N nodes, extends when the robot's remaining base drops below a threshold.
4. **The payoff experiment:** deliberately drop one order update in transit (add an `if rand.Float64() < 0.3 { return }` in your publisher). Watch the robot stop cleanly at the decision point, keep its state, and resume when the next update arrives. Nothing corrupts. That's the design working.
5. **The failure experiment:** send an update whose first node is _not_ the decision point. Watch the rejection and the error.
6. **Viewer:** colour base nodes/edges solid and horizon nodes/edges dashed, so the
   base/horizon line is visible on the canvas, not just in your head. Add a "Drop next
   update" toggle button that flips the bridge's random-drop flag from Step 4, and a
   "Send bad update" button wired to Step 5 — press it and watch the rejection error
   render next to the robot instead of scrolling past in a log.

### Check yourself

- Fleet control decides mid-journey that the robot should turn left instead of right, and the left/right split is a base node. What can it actually do? (Two options — one is `cancelOrder`, what's the other?)
- Why does an existing horizon mean the robot is "not idle" for check 3?
- Why is `sequenceId` frozen once a node is released?
- Fleet control never receives a state message after publishing an update. What should it do, and what must it _not_ assume?

---

# Session 5 — State: how the robot talks back (60 min)

### Explain it to a kid

Every time something interesting happens — and at least every half minute even if nothing does — the robot fills in the same big form: where am I, what's left to do, how's my battery, anything wrong? It never says "the same as before except X". Always the whole form.

### Why one fat topic instead of many small ones (spec §6.6)

The spec gives its reasoning: fewer messages for the broker and fleet manager to handle, **and the state stays internally consistent** — every field describes the same instant. If battery, position, and order progress arrived on three topics, you'd constantly be reasoning about a robot that doesn't exist (position from t=1, battery from t=3).

The cost is real: you resend a lot of unchanged data. In a 100-robot fleet that's meaningful Wi-Fi load. That's the trade-off, and it's the right one for correctness — but note it, because it's a genuine weakness (Session 10).

### When to publish (spec §6.6)

**Event-driven, plus a ≤30 s heartbeat.** Triggering events:

- Receiving an order or an order update
- Change in `nodeStates` / `edgeStates`
- Change in `actionStates` / `instantActionStates` / `zoneActionStates`
- Change in `errors`
- Change in `operatingMode`, `driving`, `paused`
- Change in `safetyState`
- Change in `newBaseRequest`
- Change in `lastNodeId` / `lastNodeSequenceId`
- Change in the `load` object
- Change in `edgeRequests` / `zoneRequests`
- Change in `powerSupply.charging`
- Change in `maps` / `zoneSets`
  For arrays: changes _within_ an item count, not just add/remove.

Two damping rules that matter in practice:

- **Coalesce correlated events.** Driving over a node changes `lastNodeId` _and_ `nodeStates` _and_ possibly `actionStates`. Send **one** state, not three.
- The floor on message rate is `protocolLimits.timing.minimumStateInterval` — **from the factsheet** (Session 7). The robot declares its own rate limit and both sides honour it.

### What's in it — and the one clever omission (spec §6.6.1, Figure 18)

Progress is reported as:

```json
"lastNodeId": "n_B",
"lastNodeSequenceId": 2,
"nodeStates": [ /* only UPCOMING nodes */ ],
"edgeStates": [ /* only UPCOMING edges */ ]
```

**Only the last-reached node plus what remains.** Not the history.

Why? Fleet control already knows every node it sent — it doesn't need them echoed. The only genuinely new information is _the frontier_. So the state message shrinks as the order progresses, and `lastNodeId` + the remaining lists are sufficient to reconstruct everything.

(Recall from Session 3: the _first_ node of an order is never in `nodeStates`, because the robot is already on it.)

Other significant fields:

- `mobileRobotPosition` — `x, y, theta, mapId`, plus **`localized`**. If `localized` goes false, the robot must report `LOCALIZATION_ERROR` at level `FATAL` and **must not resume automatic driving**. Coordinates without a localization flag are a trap; this closes it.
- `driving`, `paused`, `operatingMode`
- `powerSupply` — `stateOfCharge`, `batteryVoltage`, `batteryCurrent`, `batteryHealth`, `charging`, `range`. Note: `charging: false` shall only be reported if the robot is **available to take orders** — the field means "ready", not merely "not drawing current."
- `loads` — what it's carrying
- `errors`, `information`
- `safetyState` — `activeEmergencyStop` (`MANUAL` / `REMOTE` / `NONE`), `fieldViolation`
- `maps`, `zoneSets`
- `actionStates`, `instantActionStates`, `zoneActionStates`
- `edgeRequests`, `zoneRequests`
- `plannedPath`, `intermediatePath`, `velocity`

### `newBaseRequest` (spec §6.6.3)

One boolean: _"my base is running short, please extend it."_ Why it exists: to prevent unnecessary braking. The robot can see it's about to reach the decision point before fleet control's timer notices. A tiny piece of upward flow control in an otherwise downward-command protocol.

### Traversal is the robot's call (spec §6.6.2)

**The robot decides when a node counts as traversed** (subject to being inside `allowedDeviationXY` and `allowedDeviationTheta`).

Why give the robot that authority? Because only the robot knows its own localization quality. Fleet control's estimate of where the robot is arrives over Wi-Fi and is already old. Making fleet control the judge would add a round trip to every node.

### Operating modes — who's holding the leash (spec §6.6.6, Tables 10–11)

| Mode            | FC in control | Orders allowed | Instant actions    | Clears order on entry                  |
| --------------- | ------------- | -------------- | ------------------ | -------------------------------------- |
| `AUTOMATIC`     | yes           | yes            | yes                | no                                     |
| `SEMIAUTOMATIC` | yes           | yes            | yes                | no — speed via HMI, steering automatic |
| `INTERVENED`    | no            | yes            | only `cancelOrder` | no                                     |
| `MANUAL`        | no            | no             | no                 | yes                                    |
| `STARTUP`       | no            | no             | no                 | yes — state may be invalid             |
| `SERVICE`       | no            | no             | no                 | yes                                    |
| `TEACH_IN`      | no            | no             | no                 | yes — e.g. operator mapping            |

Why this exists at all: a human **must** be able to grab a robot with a joystick without the fleet manager concluding it's broken and without it driving off mid-repair. In the modes that clear the order, the robot also sets `lastNodeId` to `""` — an explicit "I no longer know where I am in any order", which prevents fleet control from stitching onto a stale position.

### Clearing the order (spec §6.6.7)

Three triggers: entering `MANUAL`/`STARTUP`/`SERVICE`/`TEACH_IN`, receiving `cancelOrder`, receiving `startHibernation`. Effects are the same as cancellation (Session 4).

### Code milestone

1. `common/models/state.go`.
2. A publisher with a dirty-flag mechanism: mutating a tracked field marks dirty (an `atomic.Bool` or mutex-guarded flag), a 10 Hz `time.Ticker` loop publishes if dirty **and** `minimumStateInterval` has elapsed, plus a 30 s heartbeat. Coalescing falls out for free.
3. Shrink `nodeStates` / `edgeStates` as the simulator traverses; update `lastNodeId` / `lastNodeSequenceId`.
4. Implement `newBaseRequest` when remaining base < 2 nodes; make the fleet manager react to it.
5. **Viewer:** replace the connection-only rows from Session 2 with a real dashboard —
   one panel per robot showing `operatingMode`, `driving`, `paused`, battery
   `stateOfCharge`, `lastNodeId`, remaining node count, and error count, all updating
   live off the `state` stream. Highlight the row when `newBaseRequest` flips true, so
   you can watch the robot ask for more road before the fleet manager reacts.

### Check yourself

- Why doesn't the state contain the nodes already traversed?
- The robot is stopped, nothing changing. How often does a state arrive, and why that number?
- The robot enters `MANUAL` and back to `AUTOMATIC`. Why can't fleet control just resume the old order?
- Why does `charging: false` mean something stricter than "not charging"?

---

# Session 6 — Actions (75 min)

### Explain it to a kid

Driving is only half the job. **Actions** are the _doing_ part: lift the fork, put it down, beep, wait for a person to press a button, read a tag on the floor. And some actions need the robot to stand still, some don't. Some can happen at the same time as other actions, some insist on being the only thing going on. That's the whole idea.

### Where actions can live (spec §6.2)

- **On a node** — "when you get here, lift the fork."
- **On an edge** — "while driving this stretch, keep the warning light on." Duration-based behaviour that belongs to a path, not a point.
- **As an instant action** — "right now, regardless of your order."
- **In a zone** — entry/during/exit actions attached to a region of floor (Session 8).
  Within a list, **array order is execution order**.

### `blockingType` — a 2×2 matrix, and why (spec §6.2.2, Table 3)

Two _independent_ questions:

- May the robot keep driving while this runs?
- May other actions run in parallel with this?
  | | Parallel allowed | Parallel not allowed |
  |---|---|---|
  | **Driving allowed** | `NONE` | `SINGLE` |
  | **Driving not allowed** | `SOFT` | `HARD` |

- `NONE` — allows driving and other actions (e.g. turn on a light)
- `SINGLE` — allows driving, no other actions
- `SOFT` — allows other actions, no driving
- `HARD` — the only allowed thing at that time (e.g. fork lift)
  Realising it's a matrix rather than a severity scale is the whole trick. Once you see it, you'll never mix `SINGLE` and `SOFT` up again.

### The queue algorithm (spec §6.2.2, Figure 11)

On reaching a node, edge, or action zone, actions are enqueued **in array order**, and:

1. If any action in the queue is `SOFT` or `HARD` → **stop automatic driving.**
2. Collect actions for parallel execution while their blocking type is `NONE` or `SOFT`.
3. When a `SINGLE` or `HARD` action comes up → all collected parallel actions must be `FINISHED` or `FAILED` **before** it starts.
4. Remove `FINISHED` / `FAILED` actions from the queue.
5. When no `SOFT` or `HARD` remains in the queue → **resume automatic driving.**
   Note the ordering of concerns: the _driving_ decision is made by scanning the whole queue, while the _parallelism_ decision is made incrementally at the head. That's why step 1 looks ahead but step 3 doesn't.

### `actionId` and the correlation contract

Every action carries an `actionId` — globally unique, UUIDs suggested. It's the key that maps the action fleet control sent to the `actionState` the robot reports back. Fleet control _must_ generate it, because fleet control is the one that needs to correlate.

The one exception: **zone actions**, where the robot generates the `actionId` itself — because fleet control never sent that specific instance; it only declared "anything entering this zone does X."

### Action state machine (spec §6.6.9, Figures 11 & 21)

```
WAITING ──► INITIALIZING ──► RUNNING ──┬──► FINISHED
                                       ├──► FAILED
                                       ├──► RETRIABLE ──► RUNNING (retry)
                                       │                └► FAILED (skipRetry)
                                       └──► PAUSED ──► RUNNING
```

- `WAITING` — received but not started. **All horizon actions are reported `WAITING` at all times** (§6.6.9). Fleet control can therefore see what _will_ happen before releasing it.
- `INITIALIZING` — optional; may be omitted if the robot transitions instantly.
- `PAUSED` — only for actions with `pauseAllowed: true`.
- `RETRIABLE` — only if the action was sent with `retriable: true` (default `false`). Then fleet control (or a human) can send the `retry` instant action with that `actionId`, or `skipRetry` to force it to `FAILED`.
  Why `RETRIABLE` exists: a fork that missed the pallet by 2 cm shouldn't fail the whole transport order. It should stop, say "I need another go", and let something with more context decide. Without it, every recoverable failure escalates to order cancellation.

Important horizon rule: horizon `actionStates` are removed if that part of the horizon is changed or dropped by an update. Base `actionStates` are **never** removed — the base can't be modified. Idea #3 again.

### Predefined actions (spec §6.2.3, Table 4)

**Mandatory for every robot:** `cancelOrder`, `startPause`, `stopPause`. That's the minimum: stop the job, pause, resume.

A representative selection, with scopes:

| Action                                    | Counter-action | Scope         | Note                                                                                                                                    |
| ----------------------------------------- | -------------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `startPause` / `stopPause`                | each other     | instant       | Linked to the `paused` state field — because many robots also have a physical pause button, and `stopPause` can release _that_ too      |
| `startCharging` / `stopCharging`          | each other     | instant, node | Linked to `powerSupply.charging`                                                                                                        |
| `startHibernation` / `stopHibernation`    | each other     | instant       | Clears the order; robot stays MQTT-connected, stops sending state, reports `HIBERNATING`                                                |
| `pick` / `drop`                           | each other     | node, instant | Load handling, with `loadId`, `height`, etc.                                                                                            |
| `detectObject`                            | —              | node, edge    |                                                                                                                                         |
| `waitForTrigger`                          | —              | node, zone    | `triggerType` array; predefined `FLEET_CONTROL` and `LOCAL`. **Fleet control owns the timeout** and must cancel the order if it expires |
| `trigger`                                 | —              | instant       | Fleet control releasing a `waitForTrigger`                                                                                              |
| `retry` / `skipRetry`                     | —              | instant       | Act on a `RETRIABLE` action by `actionId`                                                                                               |
| `initPosition`                            | —              | instant, node | Tell the robot where it is                                                                                                              |
| `factsheetRequest`                        | —              | instant       | "Introduce yourself"                                                                                                                    |
| `downloadMap` / `enableMap` / `deleteMap` | —              | instant       | Session 8                                                                                                                               |
| `downloadZoneSet` / `enableZoneSet`       | —              | instant       | Session 8                                                                                                                               |
| `updateCertificate`                       | —              | instant       | Rotate MQTT credentials                                                                                                                 |
| `cancelOrder`                             | —              | instant       | Session 4                                                                                                                               |

Each predefined action declares whether it's **idempotent**, its **parameters**, its **linked state field**, and its allowed **scopes** (instant / node / edge / zone). The linked-state idea is worth pausing on: `startPause` doesn't just do a thing, it moves a _field_ in the state. So the robot's condition is always readable, whether it was paused by fleet control or by someone's thumb on a button.

### The extensibility contract (spec §6.2.3)

> Use the predefined action **if your capability maps to its description**. Use the defined parameters if there's a sensible way to. Add parameters if you need them. If nothing maps, **define your own action** — and fleet control shall use it.

This is the pragmatic heart of VDA 5050. No committee can enumerate every gripper, so the standard says: converge where you can, extend where you must, **and declare your extensions in the factsheet.** Vendor-specific actions aren't a violation — they're the designed escape hatch. (They're also the main practical limit on interoperability. Session 10.)

### Instant actions (spec §6.2.1)

- Published to the `instantActions` topic.
- **`blockingType` is always `NONE`.** They must not stop driving on their own account.
- They **shall not conflict** with the current order — no "lower the fork" while the order says raise it. Enforcing this is fleet control's job; the spec just forbids it.
- Reported in `instantActionStates`, separate from order `actionStates` — different lifecycles, different owners.
- Unsupported → error `INVALID_INSTANT_ACTION`, level `WARNING`, with the `actionId` as `errorReference`.
  Why `WARNING` and not something scarier? Because a rejected instant action doesn't stop the robot doing its job. The level taxonomy is about _the robot's ability to continue_, not about how annoyed you should be.

### Code milestone

1. `common/models/action.go`.
2. `robot/action_engine.go` — implement Figure 11 exactly: a queue, a parallel-execution set, a `drivingAllowed` flag derived by scanning the queue.
3. Three fake actions: `pick` (`HARD`, 3 s), `drop` (`HARD`, 3 s), `beep` (`NONE`, 1 s).
4. Full `actionStatus` lifecycle in `actionStates`, including horizon actions reported `WAITING`.
5. Instant action handler for `cancelOrder`, `startPause`, `stopPause` — with `paused` reflected in state.
6. Make `pick` fail 30% of the time with `retriable: true`, and drive it to `FINISHED` via a `retry` instant action.
7. **Test the matrix:** put `[beep(NONE), pick(HARD), beep(NONE)]` on one node. Trace it. Now `[beep(SOFT), pick(SOFT)]`. Predict, then verify.
8. **Viewer:** an action-queue strip under the robot's dashboard row — each action as a
   chip showing `actionId` (short form) and `actionStatus`, colour-coded, reordering
   live as the queue drains. Add `cancelOrder` / `startPause` / `stopPause` / `retry`
   buttons per robot, wired straight to `instantActions` — this is where the panel
   stops being a viewer and starts being a control panel.

### Check yourself

- Why is an instant action's blocking type always `NONE`?
- Node actions `[A(NONE), B(SINGLE), C(SOFT)]`. When does the robot stop driving, what runs in parallel, and in what order does everything execute?
- Why do horizon actions get reported as `WAITING` instead of being kept secret until released?
- Who invents `actionId` for a zone action, and why is that the exception?

---

# Session 7 — Factsheet: self-description (45 min)

### Explain it to a kid

When a new kid joins the class, they hand the teacher a card: _"I'm this tall, I can carry this much, I understand these words, please don't talk to me faster than twice a second, and don't give me a list longer than ten things."_ Now the teacher knows how to talk to them without guessing.

### Why this exists — the point people miss

A shared _grammar_ isn't enough for interoperability. You also need to discover the other party's _capabilities and limits_. Without a factsheet, every robot model would have to be hand-configured into the fleet manager — which is exactly the integration cost VDA 5050 set out to eliminate. The factsheet is what turns "we speak the same protocol" into "we can plug in and go."

It's **mandatory**, published on the `factsheet` topic, on startup and on `factsheetRequest`.

### Contents (spec §6.10, §7.10)

- **`typeSpecification`** — series name, robot class, load capability, navigation type, max speed, agility.
- **`physicalParameters`** — length, width, height, speed and acceleration limits.
- **`protocolLimits`** — the pragmatic bit:
  - `maximumStringLengths` — max MQTT message length, max serial-number length in topics, max ID length, and an `idNumericalOnly` flag. The spec painstakingly lists _every affected field_.
  - `maximumArrayLengths` — max `order.nodes`, `order.edges`, `node.actions`, `edge.actions`, `actions.actionParameters`, `instantActions`, `trajectory.knotVector`, `trajectory.controlPoints`, `zoneSet.zones`, `state.nodeStates`, `state.edgeStates`, `state.loads`.
  - `timing` — including **`minimumStateInterval`** (the state-rate floor from Session 5).
- **`protocolFeatures`**
  - `optionalParameters` — each with support level `SUPPORTED` / `REQUIRED` / `NOT_SUPPORTED`.
  - `mobileRobotActions` — **every** action the robot supports, standard and vendor-specific, each with `actionScopes` (`INSTANT`/`NODE`/`EDGE`/`ZONE`), typed `actionParameters`, possible `blockingTypes`, `pauseAllowed`, `cancelAllowed`.
- **`mobileRobotGeometry`** — wheel definitions (type, driven, steered, position, diameter, width, `centerDisplacement` for casters), envelopes and contours.
- **`loadSets`** — named load configurations with their own limits.
- **`mobileRobotConfig`** — network and version info.

### Why bother specifying maximum lengths?

Because the other end is often an embedded controller with fixed buffers, written in C, with no dynamic allocation. Send it a 300-node order and it doesn't reject it gracefully — it corrupts memory. The factsheet lets the robot say "80 nodes maximum" and lets the fleet manager _pre-emptively split orders_. That's why the spec bothers to enumerate the affected fields one by one.

### `pauseAllowed` / `cancelAllowed` earn their keep here

Back in Session 4 you implemented: _a running action that can't be cancelled reports `RUNNING` until it's done._ Where does the fleet manager learn which actions those are? Here. This is the sort of cross-reference that makes the spec feel like a system rather than a list.

### Code milestone

1. `common/models/factsheet.go`.
2. Robot publishes it on connect and on `factsheetRequest`.
3. **Make the fleet manager actually use it.** Before publishing an order, validate against the received factsheet:
   - Reject/split orders exceeding `maximumArrayLengths.order.nodes`.
   - Reject actions not in `mobileRobotActions`.
   - Reject an action used outside its declared `actionScopes` (e.g. `pick` on an edge).
   - Reject a `blockingType` the robot didn't declare.
   - Honour `minimumStateInterval` when validating the robot's state rate.
4. Set `maximumArrayLengths.order.nodes = 4` and watch your dispatcher split a 12-node route into stitched sub-orders **automatically**. That's the factsheet and base/horizon working together — and it's the moment the design feels coherent.
5. **Viewer:** a "Factsheet" tab per robot rendering the raw limits and
   `mobileRobotActions` list, plus a `factsheetRequest` button. When the dispatcher
   splits an order (Step 4), show the sub-order boundaries as ticks along the route on
   the canvas, so the split is visible, not just present in the MQTT log.

### Check yourself

- Why is the factsheet mandatory when `zoneSet` and `visualization` are optional?
- A robot declares a vendor-specific `openGripper` action. What must the fleet manager do to use it, and what does that tell you about the ceiling on plug-and-play interoperability?
- Your fleet manager wants 10 Hz state updates. The factsheet says `minimumStateInterval: 500`. Who wins, and why is that the right answer?

---

# Session 8 — Maps, zones, requests & responses (60 min)

### Explain it to a kid

**Maps** are which floor plan you and the teacher are both looking at — one per floor of the building. **Zones** are coloured patches painted on that plan, each with a rule: _don't go here at all; go slow here; only one kid in here at a time; you must ask before entering here._

### Maps (spec §6.3, Figure 13)

A map has `mapId`, `mapVersion`, `mapStatus` (`ENABLED` / `DISABLED`), and `mapDownloadUrl`. The state's `maps` array reports which maps the robot holds.

Lifecycle via instant actions: `downloadMap` → `enableMap` → `deleteMap`.

**Why the map file itself doesn't travel over VDA 5050:** maps are megabytes. MQTT is the wrong pipe. So the robot gets a _URL_ and fetches from a **map server** that is deliberately outside the interface. VDA 5050 orchestrates the map lifecycle without carrying the payload — a nice separation, and a reminder of how narrow the interface's scope is on purpose.

Two rules with the same reasoning:

- A given `mapId` (+ version) must always mean the same content. New content → new ID. Same for `zoneSetId`.
- Re-sending an existing one → error `DUPLICATE_MAP` / `DUPLICATE_ZONE_SET`, level `WARNING`, held long enough for fleet control to notice.
  Why: it makes IDs safely **cacheable**. If content could change under a stable ID, every reference in every order becomes ambiguous, and neither side could ever be sure they're talking about the same floor.

Coordinates are one **project-specific global coordinate system**, with every map sharing the same origin (§6.3, Figure 12). Elevators are handled beautifully: the robot **disappears from the departure floor's map and spawns on the corresponding lift node of the target floor's map.** No 3D, no transitional state — just two `mapId`s and a hand-off.

### Zones (spec §6.4)

A **zoneSet** is bound to one `mapId`, has a `zoneSetId`, and contains `zone` objects. Rules:

- A zone is a **polygon**: ≥3 vertices, given **counterclockwise**, **simple** (no self-intersections), assumed closed.
- Zones **shall not overlap** — behaviour is undefined if they do.
- Zones must not extend beyond the map's boundaries.
- Several zone sets may exist per map, but **only one active per `mapId`**.
- A newly received zone set arrives `DISABLED` and must be turned on with `enableZoneSet`. Same pattern as maps: **receive and activate are separate steps**, so a zone set can be staged and switched atomically rather than taking effect mid-drive.

### The ten zone types (spec §6.4.1)

| Type                     | Meaning                                                         |
| ------------------------ | --------------------------------------------------------------- |
| `BLOCKED`                | Do not enter. Violation → `BLOCKED_ZONE_VIOLATION`, `CRITICAL`  |
| `LINE_GUIDED`            | Behave as line-guided in here, even if you're a free navigator  |
| `RELEASE`                | **Ask permission before entering**                              |
| `COORDINATED_REPLANNING` | Free navigators must get their planned path approved in here    |
| `SPEED_LIMIT`            | `maximumSpeed` applies                                          |
| `ACTION`                 | `entryActions` / `duringActions` / `exitActions`                |
| `PRIORITY`               | `priorityFactor` — bias routing toward this area                |
| `PENALTY`                | `penaltyFactor` — bias routing away                             |
| `DIRECTED`               | One-way, per `direction` + `directedLimitation`                 |
| `BIDIRECTED`             | Two-way along an axis, per `direction` + `bidirectedLimitation` |

Two triggering models (§6.4.1, Figures 14–15), and the distinction matters:

- **Contour-based** — the zone applies when any part of the robot's outline (including its **load's** extended bounding box) is inside. Use for physical constraints: `BLOCKED`, height limits.
- **Kinematic-centre-based** — applies based on the robot's control point. Use for behavioural rules where you want a crisp, unambiguous trigger.
  A loaded robot is bigger than an empty one. If the zone type didn't say which model it uses, two vendors would disagree about whether a robot with an overhanging pallet is "in" the zone. This spells it out.

Precedence note (§6.4.4): **an edge `trajectory` overrides `DIRECTED` and `BIDIRECTED` zones.** Explicit instruction beats ambient rule.

### The request/response mechanism (spec §6.4.3, §6.9, Figure 22)

This is the seam where **robot autonomy meets central authority**, and it's the newest conceptually interesting part of the protocol.

Everywhere else, fleet control commands and the robot reports. Here the robot _asks_:

```
Robot                                    Fleet Control
  │                                            │
  │ state.zoneRequests[] += {                  │
  │   requestId, zoneId,                       │
  │   requestType: ACCESS | REPLANNING,        │
  │   requestStatus: REQUESTED,                │
  │   trajectory (for REPLANNING) }            │
  ├───────────────────────────────────────────►│
  │                                            │ decides
  │◄───────────────────────────────────────────┤
  │ responses[] = { requestId,                 │
  │   grantType: GRANTED | QUEUED              │
  │             | REJECTED | REVOKED,          │
  │   leaseExpiry }                            │
  │                                            │
  │ requestStatus → GRANTED, proceed           │
  │ keep request in state while still needed   │
  │ remove it when finished                    │
```

Key mechanics and their reasons:

- The request lives **in the state message**; the answer comes on the **separate `responses` topic**. Why asymmetric? Because the request is _part of the robot's condition_ — it persists, it's re-sent with every state, and it self-heals if a message is lost. The answer is a one-shot event.
- `requestId` must be **unique across all requests** (zone _and_ edge) from that robot. May be reused after restart.
- The robot **must request before entering**, _even if the order contains released nodes inside the zone._ Base release and zone release are different currencies. Worth pausing on — it's the clearest sign that zones are a second, independent coordination layer.
- **If the response doesn't arrive in time, the robot does not enter.** Fail-safe by default.
- The robot chooses _when_ to ask, and may issue **several alternative requests at once** — e.g. three candidate trajectories through a `COORDINATED_REPLANNING` zone, letting fleet control pick.
- `QUEUED` means "heard you, not yet, you're in line." Without it, the robot can't distinguish "still thinking" from "lost message."
- **`leaseExpiry`** — grants are time-limited. Why: if the robot dies inside a `RELEASE` zone, the reservation must not be held forever. Leases make the system self-healing without a central garbage collector.
- Losing a release (expiry or `REVOKED`) → error `RELEASE_LOST`, `CRITICAL`, and the robot executes the zone's `releaseLossBehavior`.
- If the workspace is covered by two `RELEASE` zones, the robot needs **all** grants before entering. (Textbook deadlock hazard — and note the spec doesn't solve it, because deadlock resolution is out of scope. Session 10.)

### Corridors — the same machinery on edges (spec §6.1.6, §6.6.10, Figure 10)

An edge's `corridor` defines left/right widths around its trajectory within which the robot may deviate to dodge an obstacle, plus `corridorRefPoint`, `releaseRequired`, and `releaseLossBehavior`.

If `releaseRequired: true`, the robot adds an **`edgeRequest`** (`requestType: CORRIDOR`, referencing `edgeId` + `sequenceId`) and waits. Fleet control releases corridors **only for base edges**. Until granted, the robot stays exactly on its trajectory — a line-guided robot, in effect. Once granted, it may manoeuvre. When it no longer needs the corridor it **removes the request from its state**, and is line-guided again. Reaching the end of the current corridor mid-avoidance? Stop at the border, request the next one, wait.

Why this is a nice piece of design: it gives a line-guided robot **obstacle avoidance without giving up central traffic control.** The robot gets local freedom, in a bounded area, on a lease, with a defined fallback. That's the philosophical direction VDA 5050 v2 → v3 has been travelling.

### Error handling in zones (spec §6.4.5)

If the robot realises it cannot reach a node: `NODE_UNREACHABLE`, level `CRITICAL`. And then — the rule that reveals the whole philosophy — **the robot shall not try again. It waits for instructions.** Autonomy is bounded. Re-planning is fleet control's job.

### Code milestone

1. `common/models/zone.go` — zone, zoneSet, vertex structs; polygon validation (≥3, simple, counterclockwise).
2. Fleet publishes a `zoneSet`; robot stores it `DISABLED`; `enableZoneSet` activates it. Test the `DUPLICATE_ZONE_SET` path.
3. Ray-casting point-in-polygon. Report `zoneSets` in state.
4. Enforce `SPEED_LIMIT` (clamp simulator speed) and `BLOCKED` (refuse entry, raise `BLOCKED_ZONE_VIOLATION`).
5. **One full `RELEASE` round trip:** robot adds `zoneRequest` on approach, holds at the boundary, fleet responds `QUEUED` then `GRANTED` with a 10 s `leaseExpiry`, robot enters, robot removes the request on exit.
6. Let the lease expire mid-zone. Watch `RELEASE_LOST` and `releaseLossBehavior`.
7. **Viewer:** draw zone polygons on the canvas, colour by type (`BLOCKED` red,
   `RELEASE` amber, etc.). Show `zoneRequests`/`responses` as a small badge on the
   requesting robot (`REQUESTED` → `QUEUED` → `GRANTED`, with a countdown to
   `leaseExpiry`) and a manual "Grant" / "Reject" / "Revoke" button per pending
   request, so you play fleet control's decision by hand instead of hardcoding it.

### Check yourself

- Why is the request in `state` but the response on its own topic?
- Why must the robot ask for a `RELEASE` zone even when its base already includes nodes inside it?
- Why do grants expire?
- A robot needs two overlapping `RELEASE` zones and another robot needs the same two in the opposite order. What happens, and whose problem is it?
- Why must a re-sent `zoneSetId` be rejected rather than overwriting the old one?

---

# Session 9 — Visualization, planned paths, errors (45 min)

### Why a separate `visualization` topic exists (spec §6.7)

You already have position in `state`. Why a second topic?

Because **different consumers want different rates and pay different costs.** The `state` message is large and consumed by fleet _logic_ — it must be complete and consistent. A UI wants position at 10–30 Hz and doesn't care about `actionStates` or `factsheet` limits. Pushing `state` to 30 Hz to satisfy a monitor would flood the network with data the logic doesn't need.

So: `visualization` carries only `mobileRobotPosition`, `velocity`, `plannedPath`, `intermediatePath` — plus **`referenceStateHeaderId`**, tying it to the state message it belongs to. It's optional, QoS 0, and the update rate is left to the integrator. Notice that the field structures are _identical_ to their `state` counterparts — no second schema to maintain.

### Sharing planned paths (spec §6.8)

Here's the tension: fleet control does traffic management. But a **freely navigating** robot picks its own trajectory. How can fleet control coordinate a path it didn't choose?

Answer: the robot must **volunteer its intent.** A freely navigating robot shall share, in every state:

- **`plannedPath`** — a **NURBS** curve, at least covering its current base, optionally listing the `nodeId`s it will traverse. Updated on significant change. Long-range intent.
- **`intermediatePath`** — a **polyline** of waypoints, each with `x`, `y`, optional `theta`, and an **`eta`** timestamp. Only as far as its sensors can perceive. Updated with _every_ state or visualization message, always starting from the current position.
  Why two, and why different representations? `plannedPath` is a smooth, compact, exact commitment — NURBS is the right tool. `intermediatePath` is short, changes constantly, and its value is the **ETA per waypoint** — which is what lets fleet control reason about _when_ two robots will be in the same place, not just whether. A polyline with timestamps is the right tool for that.

This is VDA 5050 adapting to autonomous robots: it can't dictate the path, so it demands **legible intent** instead.

### Errors (spec §6.6.5)

```json
{
  "errorType": "NODE_UNREACHABLE",
  "errorLevel": "CRITICAL",
  "errorReferences": [{ "referenceKey": "nodeId", "referenceValue": "n_B" }],
  "errorDescription": "Path permanently obstructed by pallet",
  "errorHint": "Clear aisle 3 and send a new order",
  "errorDescriptionTranslations": [
    { "translationKey": "de", "translationValue": "..." }
  ]
}
```

**Four levels, defined by what the robot can still do** — not by how alarming they sound:

| Level      | Attention                                     | Continue current order? | Accept new orders? |
| ---------- | --------------------------------------------- | ----------------------- | ------------------ |
| `WARNING`  | not immediate; may self-resolve (dirty LiDAR) | yes                     | yes                |
| `URGENT`   | immediate (low battery)                       | yes                     | yes                |
| `CRITICAL` | immediate (picked at nothing)                 | **no**                  | yes                |
| `FATAL`    | user intervention (lost localization)         | **no**                  | **no**             |

That's the right axis. Fleet control doesn't need to know how you feel about the error; it needs to know whether it can still dispatch to you.

And the rule that ties back to Idea #3: **regardless of level, the robot shall never clear its order because of an error.** Only the explicit triggers from Session 5 clear an order. An error is information, not a command.

`errorReferences` may include `headerId`, the topic, `orderId` + `orderUpdateId`, `actionId`, or offending parameters. Descriptions and hints support **translations** by ISO 639-1 code — because the person clearing the aisle reads a local-language HMI, not the integrator's English.

Selected predefined error types (spec §6.6.5.4, Table 9), each with a mandated level, reference, and **report duration**:

`VALIDATION_FAILURE`, `INVALID_ORDER`, `OUTDATED_ORDER_UPDATE`, `SAME_ORDER_UPDATE_ID`, `ORDER_UPDATE_FOLLOWING_CANCEL`, `NO_ORDER_TO_CANCEL`, `UNSUPPORTED_PARAMETER`, `OUTSIDE_OF_CORRIDOR`, `DUPLICATE_MAP`, `DUPLICATE_ZONE_SET`, `BLOCKED_ZONE_VIOLATION`, `RELEASE_LOST`, `ZONE_ACTION_CONFLICT`, `NODE_UNREACHABLE`, `LOCALIZATION_ERROR`, `UNKNOWN_MAP_ID`, `NO_ROUTE_TO_TARGET`, `OTHER_ORDER_ACTIVE`, `START_NODE_OUT_OF_RANGE`, `MOBILE_ROBOT_NOT_AVAILABLE`.

That last column — **report duration** — is easy to skim past and important: "until a new order is accepted", "until the robot is no longer violating the zone", "long enough for fleet control to notice." Errors have _lifetimes_, because on a lossy channel a one-shot error announcement can be missed entirely. Persist it in state until it's no longer true.

**Every enum in this spec is extensible.** `errorType`, wheel `type`, `triggerType`, action types. The `…` at the end of an enum list is deliberate. Your parser must handle unknown values without crashing — plan for that in your models from the start.

### `information` (spec §6.6.4)

Free-form array, `infoLevel` `DEBUG` or `INFO`, with the same health warning as `orderDescription`: **fleet control shall not use it for logic.** Visualization and debugging only.

### Code milestone

1. `visualization` publisher at 10 Hz with `referenceStateHeaderId`.
2. `intermediatePath` with real ETAs from your simulator's speed.
3. Error injection: force `LOCALIZATION_ERROR` (`FATAL`) and confirm the robot stops driving, sets `localized: false`, refuses new orders — **and keeps its order**.
4. Make your enum types (plain `string` types with named constants, not a closed Go enum) tolerate unknown values on unmarshal instead of erroring.
5. **Viewer:** subscribe the bridge to `visualization` too and draw `plannedPath` as a
   smooth curve and `intermediatePath` as a dashed polyline with its `eta`s labelled
   at each waypoint, distinct from the actual driven trace already on the canvas since
   Session 3. Surface `errors` as a toast/badge on the robot's panel with its
   `errorLevel` colour — trigger Step 3 and watch the `FATAL` badge appear and the
   robot marker freeze in place while its dashboard still shows the held order.

### Check yourself

- Why not just publish `state` at 30 Hz and delete the `visualization` topic?
- Why is `plannedPath` a NURBS but `intermediatePath` a polyline?
- Battery at 5%. Which error level, and why not `CRITICAL`?
- Why does an error never clear the order?

---

# Session 10 — Strengths, weaknesses, and the capstone (60 min)

## Strengths

**It solved the actual problem.** N×M → N+M. This is not theoretical: multi-vendor fleets under one fleet manager are now routine, and VDA 5050 is the de facto standard in the AGV/AMR space.

**It's genuinely cheap to implement.** MQTT + JSON. A competent engineer gets a minimal compliant robot talking in days. Compare with an OPC UA information model. Low adoption cost is _why_ it won.

**Base/horizon is a genuinely elegant answer to an unreliable channel.** Append-only, immutable-once-released, self-verifying via stitching, degrades to a clean stop instead of a corruption. It could be a case study in protocol design.

**The factsheet enables real auto-configuration.** Limits, geometry, and capabilities are discoverable rather than hand-configured — which is the difference between "standard protocol" and "plug and play."

**Honest, well-drawn scope.** Excluding safety and traffic management looks like a cop-out until you notice it's what makes the standard both implementable and adoptable. It doesn't pretend a JSON message can be a safety function.

**Open governance.** Public GitHub, published JSON schemas, semantic versioning, public issue tracking. Not a document you buy and hope.

**Domain-independent by design.** Line-guided and freely navigating, forklift and tugger, 200 kg and 20 t — one interface.

## Weaknesses — the honest list

**1. The hard part is out of scope.** Traffic management is where fleets succeed or fail, and it's explicitly excluded. Two compliant products can talk perfectly and still deadlock, because deadlock resolution is unspecified. **"VDA 5050 compliant" does not mean "will cooperate well."**

**2. No security.** "Protocol security needs to be taken into account by broker configuration, but is not addressed within this guideline." So an unauthenticated broker means anyone on the network can drive every robot in the building. TLS and ACLs are entirely the integrator's problem, and `updateCertificate` is the only nod toward credential lifecycle. In 2026 this is the weakest part of the standard.

**3. Extensibility is the interoperability ceiling.** Vendor-specific actions and extensible enums are necessary — and they mean a vendor's most valuable capabilities are exactly the ones outside the standard. You still need per-vendor integration work; the factsheet just makes it discoverable rather than undocumented. Plus a long tail of optional topics and optional fields means compliance is a spectrum. Interop testing remains mandatory.

**4. Centralised by construction.** One fleet manager, star topology, no peer-to-peer. Simple and predictable, and a single point of failure. No standardized fleet-manager redundancy or handover.

**5. The chattiness of full-snapshot state.** Correct, but a 100-robot fleet publishing large JSON snapshots on every event stresses industrial Wi-Fi. `minimumStateInterval` and the `visualization` split are mitigations, not solutions. JSON verbosity doesn't help.

**6. Master/slave fits autonomous robots awkwardly.** The model was born for line-guided AGVs where central control is natural. Modern AMRs plan their own paths, and the protocol has been retrofitting: corridors, `COORDINATED_REPLANNING`, `plannedPath`/`intermediatePath`, the request/response mechanism. These are good additions, and they're _additions_ — you can feel the seam.

**7. Version churn with breaking changes.** 1.1 → 2.0 → 2.1 → 3.0, each major version breaking. Real fleets run mixed versions for years. Putting the version in the topic is a smart mitigation, but the ecosystem lags the spec by a long way.

**8. No task-level semantics.** "Transport pallet A to rack 12" doesn't exist in VDA 5050. Only nodes and edges. Order decomposition stays proprietary to each fleet manager — which is fine, but it means VDA 5050 sits lower in the stack than newcomers usually assume.

**9. Undefined behaviour at the edges.** Overlapping zones are simply "not defined." Instant actions "shall not conflict" with the order, with no conflict resolution specified. These are places where two correct implementations can diverge.

## Where it sits in the landscape

- **ISO 3691-4** — safety of driverless industrial trucks. The layer VDA 5050 deliberately doesn't touch.
- **VDA 5050** — the fleet-control ↔ robot wire.
- **MassRobotics AMR Interoperability Standard** — overlapping goal, more focused on robot-to-robot status sharing; less adopted.
- **OPC UA / OPC UA for AGVs** — richer information modelling, heavier, more common upward toward MES/ERP.
- **VDMA 15276 / VDA 5050 Fleet Manager interfaces** — work on standardizing the layer _above_, i.e. WMS ↔ fleet manager, which is where the task-level gap is being addressed.
  Mental model: **VDA 5050 is TCP for robot fleets.** It moves the right bytes reliably. It says nothing about what you should do with them.

## Capstone (30 min of the session)

Run **one fleet manager + two robots + one shared corridor**, and implement the simplest possible traffic rule:

1. Two robots, opposite ends, routes crossing on a single shared edge.
2. Fleet control releases the shared edge's nodes to **one robot only**, holding the other at its decision point with the shared segment sitting in its **horizon**.
3. When robot 1's state shows it has passed the far node, extend robot 2's base through the corridor.
4. Now do it again using a `RELEASE` zone over the corridor instead — robots ask, fleet control grants one at a time with a lease.
5. **Viewer:** run this entirely through the panel you've been building since Session
   2 — both robots on one canvas, both dashboards side by side, the held robot's
   decision point highlighted and its horizon dashed through the shared corridor. Use
   the "Grant" button from Session 8 to release the corridor by hand the first time,
   then let the fleet manager do it automatically and just watch.
   Watch what you've built: no collision, no deadlock, and **the coordination mechanism is nothing but the base/horizon release you built in Session 4.** That's the payoff. Fleet control never told either robot to stop. It simply didn't give one of them permission to continue — and the protocol turned an absence of permission into a clean, safe halt.

Then reflect: you just wrote traffic management. It's about 30 lines and it's _terrible_ — it doesn't handle three robots, priorities, or two corridors. Now you understand viscerally why the spec left this out, and why fleet-manager vendors have entire teams on it.

## Final self-test — can you explain, unaided?

1. Why can't the base be changed, and name four features that exist because of it.
2. Why does a state message contain the remaining nodes but not the traversed ones?
3. What does `sequenceId` do that `nodeId` can't?
4. Draw the `blockingType` matrix and give a real example of each.
5. Why is `connection` the only QoS 1 topic?
6. Two robots need the same corridor. Give both mechanisms VDA 5050 offers, and say which layer each belongs to.
7. Why does the factsheet exist, and what breaks without it?
8. Where does VDA 5050 stop, and who picks up in each direction?
   If you can answer all eight from memory, you understand VDA 5050 better than most people shipping it.
