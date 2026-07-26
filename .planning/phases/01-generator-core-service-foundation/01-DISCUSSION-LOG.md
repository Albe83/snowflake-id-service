# Phase 1: Generator Core & Service Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-26
**Phase:** 1-Generator Core & Service Foundation
**Areas discussed:** Clock-skew policy, Epoch value & configurability, Binary scope in Phase 1, Node-ID resolution scope

---

## Clock-Skew Policy

| Option | Description | Selected |
|--------|-------------|----------|
| Reject with error | Return `ErrClockMovedBackwards`; Phase 2 maps to 503. Simplest, most honest contract — duplicates impossible, skew immediately visible, fleet absorbs load | ✓ |
| Bounded wait, then reject | Wait up to a small bound (5–10ms) for clock catch-up, then reject. Absorbs tiny NTP steps, still fails loud on big ones | |
| Monotonic-anchored | Anchor to Go's monotonic clock (bwmarrin approach): NTP steps invisible, but zero skew detection and ID timestamps drift from wall time | |
| Unbounded wait | Block until clock catches up, no timeout. Research: never acceptable — persistent backwards clock hangs requests | |

**User's choice:** Reject with error (recommended option)
**Notes:** This was the roadmap's explicit decision gate (STATE.md blocker). Resolved with the corollaries: zero tolerance (any backwards movement rejects), no state mutation on rejection (`lastMs` stays the high-water mark), observability via the error seam with counter/log wired in the metrics phase (v2). User moved to next area after one exchange — no further sub-questions needed.

---

## Epoch Value & Configurability

| Option | Description | Selected |
|--------|-------------|----------|
| 2026-01-01T00:00:00Z | 1767225600000 ms. Round, memorable, "IDs valid until ~2095" | ✓ (value) |
| 2026-07-26 (inception date) | Maximizes runway to ~2096, arbitrary-looking number | |
| You decide | Agent picks 2026-01-01 with documentation | |

**User's choice:** Free-text — *"Usa 2026-01-01T00:00:00Z come default sovrascrivibile attraverso configurazione"* (value 2026-01-01, env-overridable)
**Notes:** After the agent presented the duplicate-ID footgun of an env-overridable epoch (two instances with different epochs mint disjoint-then-colliding spaces) and proposed guardrails, the user **reversed**: *"non rendiamolo configurabile, usiamo come costante nel codice, ma logghiamolo all'avvio."* Final decision: hardcoded constant in `internal/idgen`, no env override, boot-logged, "~2095" documented in README + constant comment.

---

## Binary Scope in Phase 1

| Option | Description | Selected |
|--------|-------------|----------|
| No HTTP in Phase 1 | Boot → validate → log → block on signal → exit 0. All verification via tests; cleanest phase boundary | |
| Temporary dev route | Minimal mux with one temp route (e.g. GET /dev/id) for local curl smoke tests; throwaway, marked "replaced in Phase 2" | ✓ |
| You decide | Agent decides in planning | |

**User's choice:** Temporary dev route
**Notes:** Follow-up question on the response shape:

| Option | Description | Selected |
|--------|-------------|----------|
| JSON string `{"id": "..."}` | Dogfoods the final contract, verifies GEN-03 end-to-end, no 2^53 risk | ✓ |
| Plain text | Simpler, proves nothing about the JSON contract | |
| You decide | Planner decides | |

Consequences recorded: `PORT` enters Phase 1 config (with default); shutdown skeleton wires `signal.NotifyContext` → `srv.Shutdown()` (observable drain stays Phase 2 per STATE.md); dev route must not calcify (`/dev/` prefix + replacement comment).

---

## Node-ID Resolution Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Ordinal parsing in Phase 1 | Config resolves NODE_ID > POD_NAME ordinal > error; Phase 3 adds manifests only | |
| Solo NODE_ID, ordinal in Phase 3 | Phase 1 reads explicit NODE_ID only; parsing lands with StatefulSet work | |
| You decide | Planner picks the boundary | |

**User's choice:** Free-text — *"Il node id viene passato come variabile d'ambiente"*
**Notes:** Ambiguity check (plain text): interpretation A = Phase 1 reads only NODE_ID (ordinal later); interpretation B = always and only explicit NODE_ID, never ordinal parsing — which amends locked OPS-04. User confirmed **B**. Recorded as an explicit amendment to OPS-04 (REQUIREMENTS.md) and Phase 3 success criteria 1–2 (ROADMAP.md): deployment model becomes N single-replica workloads with distinct NODE_ID per pod template; human-managed node-ID registry becomes THE assignment mechanism; ">1 replica per workload = guaranteed duplicates" becomes the most dangerous misconfiguration. User was shown the research's "operational toil" trade-off and chose the simpler mental model knowingly.

---

## the agent's Discretion

- `PORT` default value and env var naming details
- Exact dev-route path within `/dev/` prefix
- Clock-injection mechanism for frozen-clock/skew tests (needed by success criterion 3)
- Internal error naming, test file organization, slog setup details

## Deferred Ideas

- Skew-event counter + structured log wiring at the rejection seam → v2 metrics (METR-02); seam is Phase 1, instrumentation is not
- Observable graceful drain (readyz flip + in-flight completion, OPS-03) → Phase 2 (STATE.md boundary note)
- K8s manifests under the amended deployment model + node-ID registry → Phase 3
