<!-- GSD:project-start source:PROJECT.md -->

## Project

**Snowflake ID Service**

Microservizio distribuito scritto in Go che espone una API REST per generare ID globalmente univoci (Snowflake ID a 64 bit) senza condivisione di risorse né coordinamento tra istanze. È un componente del framework "Foundation" per l'implementazione rapida (RAD) di sistemi gestionali: funge sia da progetto di analisi hands-on su Go e vibe coding, sia da servizio che verrà riutilizzato in produzione nei progetti futuri.

**Core Value:** Generare ID globalmente univoci in modo affidabile da qualsiasi istanza, senza coordinamento — se questo fallisce (ID duplicati), tutto il resto non ha valore.

### Constraints

- **Tech stack**: Go — scelta esplicita del progetto, anche a scopo didattico
- **API**: REST — niente gRPC per ora
- **Architettura**: nessuna risorsa condivisa né coordinamento tra istanze — vincolo fondante
- **Deploy**: Kubernetes — config statica del node ID deve integrarsi col modello K8s (es. env da Deployment/StatefulSet)
- **Compatibilità**: ID a 64 bit signed per essere sicuri in JSON e nei database dei gestionali futuri

<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->

## Technology Stack

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.22+ (use latest stable; verify at planning) | Language & runtime | Mandatory per PROJECT.md. Go 1.22 is the hard floor: its `net/http.ServeMux` adds method patterns + wildcards (`POST /v1/ids`, `{id}` path values), eliminating any need for a router dependency. |
| `net/http` (stdlib) | — | HTTP server & routing | ~5 routes total. Method-aware patterns, automatic 405, conflict panics at startup. Zero supply-chain surface. Verified: go.dev blog "Routing Enhancements for Go 1.22". |
| `log/slog` (stdlib) | — | Structured logging | Stdlib structured logging since Go 1.21; JSON handler for machine-parseable pod logs; no logrus/zap dependency needed at this size. |
| Kubernetes manifests (plain YAML) | — | Deployment: StatefulSet + Service + PDB | The node-ID mechanism lives in the manifests (downward API pod name → ordinal). Plain YAML suffices for MVP; Helm/Kustomize adds tooling before there's anything to parameterize. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `prometheus/client_golang` | latest v1.x (v1.24.1 verified) | `/metrics` endpoint, per-node counters, request duration instrumentation | From the observability phase. The ONLY external runtime dependency. Prometheus is the K8s-ecosystem standard; `promhttp.Handler()` is zero-config. |
| In-house `internal/idgen` (~100 lines) | — | Snowflake 41/10/12 generator | From phase 1. See "Alternatives Considered" — implementing in-house is the deliberate choice. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go test -race` | Unit + concurrency tests | Race detector is non-negotiable for the mutex-guarded generator. Concurrency test: N goroutines × M IDs → assert global uniqueness. |
| `golangci-lint` | Linting in CI | Standard meta-linter; enable `govet`, `staticcheck`, `errcheck`, `gosec`. |
| Docker multi-stage build | Container image | `golang:<latest>` builder → `gcr.io/distroless/static-debian12` (or `scratch`) runtime. Static `CGO_ENABLED=0` binary; no shell needed in prod image. |
| `Makefile` (optional) | Task runner | `make test`, `make lint`, `make image`, `make deploy-dev`. Convenience only. |

## Installation

# Initialize module (phase 1)

# Only external dependency (observability phase)

# Dev tooling

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| In-house `internal/idgen` | `bwmarrin/snowflake` library | If speed-to-ship were the only goal. Rejected because: (a) in-house is ~100 lines and this project is explicitly a Go learning vehicle; (b) we need a custom epoch, startup `now ≥ epoch` validation (a verified gap in the library), and a clock-skew policy seam returning `(int64, error)`; (c) the library's package-level globals are marked DEPRECATED by its own author. The library remains the **design reference** (mutex model, spin-wait overflow, JSON string encoding all verified against its source). |
| stdlib `net/http` mux | chi / gin / echo | If routes grow past ~15 or need regex matching. chi drops in later with zero architectural change (still `http.Handler`). Not justified for 5 routes. |
| Plain env vars + `os.Getenv` | viper / envconfig / koanf | If config grows beyond ~6 vars or needs file/watch support. At this size a config library is ceremony; one validated `Config` struct, fail-fast at boot. |
| Plain K8s YAML | Helm / Kustomize | When Foundation projects need per-environment parameterization. Revisit when a second environment exists. |
| StatefulSet | Deployment + per-pod env | Never viable — see ARCHITECTURE.md deployment-model comparison: shared `NODE_ID` across replicas = guaranteed duplicates. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| gorilla/mux | In maintenance mode (archived); stdlib 1.22 mux covers our routing | `net/http` ServeMux |
| `pkg/` directory on day one | Declares "safe for external import" — nothing is, yet; Foundation consumes REST | `internal/` (compiler-enforced boundary); promote to `pkg/` only if in-process generation becomes a requirement |
| Atomic-CAS or channel-based generator | Format ceiling is 4096 IDs/ms regardless; alternatives multiply complexity on the two hardest paths (overflow, skew) for zero benefit | `sync.Mutex`-guarded struct (verified reference design) |
| UUID/ULID libraries | Snowflake is the explicit project choice (sortable, 64-bit, DB-friendly) | In-house `internal/idgen` |
| etcd/Redis/ZooKeeper node-ID lease | Violates the no-coordination founding constraint; adds a failure dependency | StatefulSet ordinal via downward API |

## Stack Patterns by Variant

- Explicit `NODE_ID=0` env var, skip K8s entirely, `go run ./cmd/snowflake-service`
- Because: the ordinal-parsing path only activates when `POD_NAME` is set (explicit `NODE_ID` overrides).
- `go vet ./... && golangci-lint run && go test -race ./...` then `docker build`
- Because: the generator's uniqueness property must be proven by the race-enabled concurrency test before any image ships.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| Go ≥ 1.22 | stdlib mux method patterns | Hard floor; pin `go 1.2x` in go.mod to the toolchain used in CI and in the Docker builder image — keep all three identical. |
| prometheus/client_golang v1.x | Any Go ≥ 1.20 | No known conflicts; it's the only dep, so `go.mod` stays auditable at a glance. |
| distroless/static-debian12 | CGO_ENABLED=0 binary | No libc in runtime image → static build mandatory. Verified pattern for Go. |

## Sources

- bwmarrin/snowflake source + README — generator design reference, JSON string encoding, deprecated globals (verified 2026-07-26, see ARCHITECTURE.md)
- go.dev blog — "Routing Enhancements for Go 1.22" (verified 2026-07-26)
- pkg.go.dev — `promhttp` (client_golang v1.24.1), `os/signal.NotifyContext` (verified 2026-07-26)
- Exact current Go stable version — NOT re-verified inline; confirm at phase planning (MEDIUM confidence, non-blocking)

<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
