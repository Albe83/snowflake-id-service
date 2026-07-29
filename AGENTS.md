# AGENTS.md — ID Service

## Project

Microservizio Go che genera UUIDv7 a 128 bit (48 bit timestamp + 74 bit random, RFC 9562) via REST API. Nessun coordinamento tra istanze. Deploy su Kubernetes via StatefulSet.

## Comandi

```
go test -race ./...           # race detector obbligatorio per il generatore
golangci-lint run             # lint in CI
CGO_ENABLED=0 GOOS=linux go build -o /dev/null ./...
```

Go ≥ 1.22 (per `net/http` mux con method patterns).

## Architettura

- **`internal/`** — tutto il codice; mai `pkg/` finché nessun consumatore esterno importa
- **`internal/idgen/`** — generatore UUIDv7 (~50 linee), stateless (nessun lock, nessun mutex)
- Router: **`net/http` stdlib** (Go 1.22+), niente chi/gin/echo
- Config: un `Config` struct validato via env (`os.Getenv`), fail-fast all'avvio (`log.Fatal`)
- Node ID: non necessario (74 bit random garantiscono unicità)
- Logging: `log/slog` JSON handler; niente logrus/zap
- Unica dipendenza esterna: `prometheus/client_golang` (fase osservabilità)

## Decisioni da non cambiare

- **In-house generator** (non `bwmarrin/snowflake`): ~50 righe, UUIDv7 stateless, solo `crypto/rand` + `time`, zero deps
- **ID a 128 bit**: stringa UUIDv7 canonica in JSON, non number (sicuro in JS e DB)
- **Nessun coordinamento runtime**: niente etcd/Redis/ZooKeeper per node ID
- **Stateless**, nessun lock, mutex, atomic o channel — il generatore è una funzione pura tranne `crypto/rand`

## Vincoli

- Avvio fallisce (`os.Exit(1)`) se system clock < Unix epoch 0
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
