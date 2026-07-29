# AGENTS.md — Snowflake ID Service

## Project

Microservizio Go che genera Snowflake ID a 64 bit (layout 41/10/12, epoch custom) via REST API. Nessun coordinamento tra istanze. Deploy su Kubernetes via StatefulSet.

## Comandi

```
go test -race ./...           # race detector obbligatorio per il generatore
golangci-lint run             # lint in CI
CGO_ENABLED=0 GOOS=linux go build -o /dev/null ./...
```

Go ≥ 1.22 (per `net/http` mux con method patterns).

## Architettura

- **`internal/`** — tutto il codice; mai `pkg/` finché nessun consumatore esterno importa
- **`internal/idgen/`** — generatore Snowflake (~100 linee), `sync.Mutex`-guarded struct
- Router: **`net/http` stdlib** (Go 1.22+), niente chi/gin/echo
- Config: un `Config` struct validato via env (`os.Getenv`), fail-fast all'avvio (`log.Fatal`)
- Node ID: env `NODE_ID=0` per dev locale; in K8s derivato dall'ordinal del StatefulSet
- Logging: `log/slog` JSON handler; niente logrus/zap
- Unica dipendenza esterna: `prometheus/client_golang` (fase osservabilità)

## Decisioni da non cambiare

- **In-house generator** (non `bwmarrin/snowflake`): ~100 righe, epoch custom, policy clock-skew, zero deps
- **ID 64-bit signed**: stringa JSON, non number (sicuro in JS e DB)
- **Nessun coordinamento runtime**: niente etcd/Redis/ZooKeeper per node ID
- **`sync.Mutex`**, non atomic-CAS né channel-based generator

## Vincoli

- Avvio fallisce (`os.Exit(1)`) se node ID fuori 0–1023 o system clock < custom epoch
- Graceful shutdown su SIGTERM: drain delle richieste in volo
- Immagine Docker multi-stage → `distroless/static-debian12`, binario statico
- Zero duplicati anche con N goroutine concorrenti (verificato da test race-enabled)

## Endpoint previsti

| Method | Path | Descrizione |
|--------|------|-------------|
| POST | `/v1/ids` | Genera 1 o batch (max 1000) ID |
| GET | `/healthz` | Liveness |
| GET | `/readyz` | Readiness (fallisce durante drain shutdown) |
| GET | `/metrics` | Prometheus (fase successiva) |
| GET | `/v1/ids/{id}` | Decode ID (nice-to-have) |
