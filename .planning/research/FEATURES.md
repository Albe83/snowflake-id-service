# Feature Research

**Domain:** Unique ID generation as a service (Snowflake-style, REST API)
**Researched:** 2026-07-26
**Confidence:** MEDIUM-HIGH — synthesized inline (parallel research subagents failed) from the primary-source-verified ARCHITECTURE.md plus established domain practice (Twitter Snowflake, Baidu UidGenerator, Meituan Leaf, sonyflake). Feature categorization is stable and well-known in this domain.

## Feature Landscape

### Table Stakes (Users Expect These)

Features consumers of an ID service assume exist. Missing these = service feels broken or untrustworthy.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Generate one ID via REST | The entire reason the service exists | LOW | `POST /v1/ids` (no body or `{"count": 1}`) → `{"ids": ["..."]}` |
| Batch generation (N IDs per call) | Every reference service offers it; avoids per-ID HTTP overhead for bulk insert workloads | LOW | `count` param, capped (e.g. ≤1000) to bound mutex hold time |
| IDs as JSON strings | 63-bit IDs exceed JS `Number.MAX_SAFE_INTEGER` (2^53) — numeric JSON silently corrupts them in web consumers | LOW | Verified: reference Go library's own `MarshalJSON` emits strings |
| Liveness + readiness endpoints | K8s cannot run the service safely without them; standard for any containerized service | LOW | `/healthz` (process up), `/readyz` (generator valid + not draining) |
| Graceful shutdown | "Near-zero downtime maintenance" is a stated project goal; SIGTERM must drain in-flight requests | LOW-MEDIUM | `signal.NotifyContext` + `Server.Shutdown`; readiness flips at drain start |
| Fail-fast config validation | A bad NODE_ID or pre-epoch clock must crash the pod loudly, not generate garbage silently | LOW | Boot: `0 ≤ nodeID ≤ 1023`, `now ≥ epoch`, port valid → else exit(1) |
| API versioning (`/v1/`) | ID services are long-lived infrastructure; contract evolution must be possible | LOW | Path prefix; `/v1/ids` from day one |

### Differentiators (Competitive Advantage)

Not required, but valuable — these are what make this service a solid Foundation building block rather than a toy.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Prometheus `/metrics` with per-node counters | **Proves the no-duplicates property in production**: `ids_generated_total{node_id}` makes collisions visible; skew/overflow counters surface the two silent failure modes | LOW-MEDIUM | `snowflake_clock_skew_events_total`, `snowflake_sequence_waits_total`; the only external dependency |
| Decode endpoint (`GET /v1/ids/{id}`) | Debugging superpower for gestionali: given an ID, see when/where it was generated — invaluable when IDs land in business tables | LOW | Pure bit-shift function, any pod can decode any ID; returns timestamp, node_id, sequence, ISO-8601 |
| Config introspection endpoint | Ops can verify which node ID / epoch a pod resolved without shell access — directly mitigates the #1 pitfall (duplicate node IDs) | LOW | `GET /v1/config` (or part of `/readyz` detail): node_id, epoch, version |
| OpenAPI spec | Foundation projects get a generated client for free; documents the string-ID contract | LOW-MEDIUM | Single `openapi.yaml`; keep hand-maintained, no codegen framework |
| Clock-skew observability (metric + structured log) | Turns the deferred skew strategy from silent risk into an alertable event | LOW | Counter increments on every skew detection regardless of policy |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Dynamic node-ID assignment (etcd/Redis lease, DB-backed allocator) | "No manual node bookkeeping" | Violates the no-coordination founding constraint; adds a runtime dependency whose outage = ID outage; this is UidGenerator/Leaf "snowflake mode" territory | StatefulSet ordinal + markdown registry of assigned node IDs |
| AuthN/AuthZ on the API | "All APIs need auth" | Internal cluster service; auth adds latency, config, and failure modes for zero current threat | Network-level isolation (ClusterIP, NetworkPolicy); add gateway-level auth later if exposure changes |
| Rate limiting / quotas | "Protect the service" | Low volume by design; the 4096/ms/node format ceiling is the natural limit; complexity without a threat model | Batch `count` cap; revisit if consumers multiply |
| Per-language client SDKs | "Consumers shouldn't write HTTP" | Premature: one endpoint, two verbs. SDKs multiply maintenance across Foundation languages | OpenAPI spec → consumers generate their own clients when needed |
| Admin UI / dashboard | "See IDs being generated" | A Grafana panel over the Prometheus metrics does this better with zero service code | `/metrics` + Grafana |
| Multiple ID formats (UUID, ULID alongside snowflake) | "Flexibility for consumers" | Dilutes the core value, doubles test surface, invites consumer confusion | One format, done provably right (explicit PROJECT.md out-of-scope) |

## Feature Dependencies

```
idgen core (Generate/Decode + tests)
    └──required by──> POST /v1/ids (single + batch)
    └──required by──> decode endpoint
    └──required by──> per-node generation metrics

config (node ID resolution + validation)
    └──required by──> idgen core instantiation
    └──required by──> readiness endpoint (reports resolved node ID)

health endpoints
    └──required by──> K8s manifests (probes reference them)

K8s multi-instance (StatefulSet)
    └──enhances──> per-node metrics (node_id label gains meaning)
    └──required by──> e2e duplicate-check across pods

OpenAPI spec ──documents──> POST /v1/ids contract (string IDs, batch cap, 503 on skew)

rate limiting ──conflicts──> low-volume design premise (anti-feature, deferred)
```

### Dependency Notes

- **idgen core requires nothing:** it is the dependency of everything else — therefore phase 1, with the concurrency uniqueness test as its gate.
- **Metrics enhances multi-instance but follows it:** per-node labels are only meaningful once multiple node IDs actually exist in the fleet.
- **Decode depends only on idgen core:** zero architectural risk, slot whenever convenient (declared nice-to-have → last).
- **Config introspection depends on config + httpapi both existing:** cheap trust-builder, bundle with metrics phase.

## MVP Definition

### Launch With (v1)

Minimum viable service — usable by a Foundation project in production.

- [ ] POST /v1/ids single + batch, IDs as JSON strings — the core value made callable
- [ ] /healthz + /readyz + graceful shutdown — K8s-safe lifecycle
- [ ] Fail-fast config validation (node ID range, clock ≥ epoch) — no silent garbage
- [ ] Dockerfile + StatefulSet/Service/PDB manifests — the resilience story is part of the MVP per PROJECT.md

### Add After Validation (v1.x)

- [ ] /metrics with per-node counters + skew/overflow counters — trigger: first multi-pod deployment, to *prove* uniqueness in prod
- [ ] Config introspection endpoint — trigger: first ops question "which node ID is this pod?"
- [ ] OpenAPI spec — trigger: first external consumer

### Future Consideration (v2+)

- [ ] Decode endpoint — nice-to-have per PROJECT.md, zero risk, do when convenient
- [ ] Helm/Kustomize packaging — when a second environment/consumer appears
- [ ] `pkg/idgen` promotion for in-process use — only if a Foundation project needs to skip the network hop

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| POST /v1/ids (single+batch, string IDs) | HIGH | LOW | P1 |
| Health endpoints + graceful shutdown | HIGH | LOW | P1 |
| Config validation fail-fast | HIGH | LOW | P1 |
| K8s StatefulSet manifests | HIGH (resilience is the stated goal) | MEDIUM | P1 |
| /metrics per-node counters | HIGH (proves core value) | LOW-MEDIUM | P2 |
| Config introspection | MEDIUM | LOW | P2 |
| OpenAPI spec | MEDIUM | LOW-MEDIUM | P2 |
| Decode endpoint | MEDIUM-LOW | LOW | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

## Competitor Feature Analysis

| Feature | Twitter Snowflake (original) | Baidu UidGenerator / Meituan Leaf | Our Approach |
|---------|------------------------------|-----------------------------------|--------------|
| Worker ID assignment | ZooKeeper-coordinated (in later variants) | DB/lease-allocated worker IDs | Static config: StatefulSet ordinal via downward API — zero coordination per project constraint |
| Clock skew handling | Reject/panic (implementation-defined) | Cached " RingBuffer" decouples from clock | Explicit policy seam, decision in planning (reject-503 vs bounded wait), always metric + log |
| Throughput strategy | Single generator | RingBuffer pre-fill (Leaf) for burst throughput | Not needed at low volume — mutex generator saturates the 4096/ms format ceiling anyway |
| API surface | Thrift (era-specific) | HTTP + SDKs | REST `/v1/ids` only; OpenAPI instead of SDKs |

## Sources

- ARCHITECTURE.md (this repo) — primary-source-verified patterns: string JSON encoding, spin-wait overflow, StatefulSet identity, health/metrics design (2026-07-26)
- Domain knowledge: Twitter Snowflake announcement + repo, Meituan Leaf & Baidu UidGenerator public docs (worker-ID allocation approaches), sonyflake layout rationale — MEDIUM confidence, well-established
- PROJECT.md — explicit MVP sequencing and out-of-scope boundaries

---
*Feature research for: Snowflake-style distributed ID generation service*
*Researched: 2026-07-26 (synthesized inline, consistent with primary-source-verified ARCHITECTURE.md)*
