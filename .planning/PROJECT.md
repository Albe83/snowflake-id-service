# Snowflake ID Service

## What This Is

Microservizio distribuito scritto in Go che espone una API REST per generare ID globalmente univoci (Snowflake ID a 64 bit) senza condivisione di risorse né coordinamento tra istanze. È un componente del framework "Foundation" per l'implementazione rapida (RAD) di sistemi gestionali: funge sia da progetto di analisi hands-on su Go e vibe coding, sia da servizio che verrà riutilizzato in produzione nei progetti futuri.

## Core Value

Generare ID globalmente univoci in modo affidabile da qualsiasi istanza, senza coordinamento — se questo fallisce (ID duplicati), tutto il resto non ha valore.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Endpoint REST `POST /v1/ids` che genera 1 o più ID su richiesta
- [ ] Algoritmo Snowflake ID a 64 bit con layout classico 41/10/12 (timestamp con epoch custom, node ID, sequence)
- [ ] Node ID assegnato via configurazione statica (env var / config file) — nessun coordinamento runtime
- [ ] Deployment multi-istanza su Kubernetes per resilienza, self-healing e near-zero downtime in manutenzione
- [ ] Endpoint operativi (health, metrics) per osservabilità
- [ ] Endpoint di decode di un ID (timestamp, node, sequence) — nice-to-have

### Out of Scope

- Registry dinamico per l'assegnazione dei node ID (etcd/Redis lease) — richiederebbe coordinamento, contro i vincoli del progetto
- Database o storage persistente — gli ID sono generati in-memory, stateless
- Autenticazione/autorizzazione sull'API — servizio interno al framework Foundation
- Algoritmi alternativi (UUID, ULID) — Snowflake è la scelta deliberata

## Context

- Il servizio nasce come primo mattoncino di un framework operativo (Foundation) per RAD di sistemi gestionali; verrà riusato come dipendenza nei progetti futuri.
- Doppia valenza: hands-on learning su Go e vibe coding + componente production-grade.
- Volumi di generazione attesi bassi; la multi-istanza non serve per il carico ma per resilienza, self-healing e manutenzione senza downtime.
- Approccio di sviluppo incrementale e iterativo (MVP): si parte dal solo endpoint di generazione, poi endpoint operativi (health/metrics), poi decode; ulteriori endpoint emergeranno con la maturazione.
- Il clock skew (orologio di sistema che torna indietro) va gestito, ma la strategia (rifiuto con errore vs attesa) verrà decisa in fase di pianificazione con pro/contro.

## Constraints

- **Tech stack**: Go — scelta esplicita del progetto, anche a scopo didattico
- **API**: REST — niente gRPC per ora
- **Architettura**: nessuna risorsa condivisa né coordinamento tra istanze — vincolo fondante
- **Deploy**: Kubernetes — config statica del node ID deve integrarsi col modello K8s (es. env da Deployment/StatefulSet)
- **Compatibilità**: ID a 64 bit signed per essere sicuri in JSON e nei database dei gestionali futuri

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Algoritmo Snowflake ID (64 bit) | Unicità distribuita senza coordinamento, ID ordinabili temporalmente | — Pending |
| Layout classico 41/10/12 con epoch custom | Standard Twitter, 69 anni di autonomia dall'epoch, 1024 nodi, 4096 ID/ms/nodo | — Pending |
| Node ID da configurazione statica | Rispetta il vincolo "nessun coordinamento"; semplicità MVP | — Pending |
| Sviluppo incrementale MVP (generazione → ops → decode) | Maturazione graduale, feature aggiunte quando necessario | — Pending |
| Multi-istanza per resilienza, non per carico | Volumi bassi; obiettivo è near-zero downtime in manutenzione | — Pending |
| Clock skew strategy rimandata | Scelta consapevole da fare in planning con trade-off espliciti | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-26 after initialization*
