# Requirements: Snowflake ID Service

**Defined:** 2026-07-26
**Core Value:** Generare ID globalmente univoci in modo affidabile da qualsiasi istanza, senza coordinamento — se questo fallisce (ID duplicati), tutto il resto non ha valore.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Generazione

- [ ] **GEN-01**: Il consumer può richiedere 1 ID univoco via `POST /v1/ids`
- [ ] **GEN-02**: Il consumer può richiedere un batch di ID univoci (fino a 1000) in una sola chiamata `POST /v1/ids`
- [ ] **GEN-03**: Ogni ID restituito è un intero positivo a 64 bit (layout Snowflake 41/10/12 con epoch custom) serializzato come stringa JSON
- [ ] **GEN-04**: Una richiesta batch invalida (count < 1 o count > cap) riceve risposta `400` con errore esplicito
- [ ] **GEN-05**: ID generati concorrentemente da istanze diverse sono globalmente univoci (zero duplicati, verificato end-to-end su N pod)

### Ops / Lifecycle

- [ ] **OPS-01**: L'orchestratore può verificare la liveness del processo via `GET /healthz`
- [ ] **OPS-02**: L'orchestratore può verificare la readiness via `GET /readyz`, che fallisce all'inizio dello shutdown drain
- [ ] **OPS-03**: Le richieste in volo vengono completate durante lo shutdown del pod (graceful drain su SIGTERM)
- [ ] **OPS-04**: L'operatore può deployare N repliche su Kubernetes via StatefulSet; ogni pod risolve un node ID distinto e stabile dal proprio ordinal (downward API)
- [ ] **OPS-05**: Il servizio termina con exit≠0 all'avvio se la configurazione è invalida (node ID fuori da 0–1023, clock di sistema precedente all'epoch)
- [ ] **OPS-06**: L'operatore può eseguire il servizio come container da un'immagine Docker multi-stage (binario statico, base distroless/scratch)

### Config Introspection

- [ ] **CONF-01**: L'operatore può leggere la configurazione risolta a runtime (node_id, epoch) via endpoint di introspection

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Osservabilità

- **METR-01**: L'operatore può scrapare `/metrics` Prometheus con contatore `ids_generated_total` etichettato per node_id
- **METR-02**: L'operatore può osservare contatori `snowflake_clock_skew_events_total` e `snowflake_sequence_waits_total`

### Introspection

- **DEC-01**: Il consumer può decodificare un ID via `GET /v1/ids/{id}` (timestamp, node_id, sequence, ISO-8601)

### Documentazione & Packaging

- **DOC-01**: Il consumer può consultare una specifica OpenAPI dell'API
- **PKG-01**: L'operatore può installare il servizio via Helm chart (quando esiste un secondo ambiente)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Registry dinamico node ID (etcd/Redis lease, allocazione DB) | Viola il vincolo fondante "nessun coordinamento"; StatefulSet ordinal lo risolve staticamente |
| Autenticazione/autorizzazione sull'API | Servizio interno al cluster (ClusterIP); esposizione esterna = decisione futura a livello gateway |
| Persistenza / database | Generazione stateless in-memory; nessuno stato da salvare |
| Formati alternativi (UUID, ULID) | Snowflake è la scelta deliberata (PROJECT.md Key Decisions) |
| Rate limiting / quote | Volumi bassi by design; il cap sul batch è il limite naturale |
| SDK client per linguaggio | Un endpoint, due verbi: OpenAPI (v2) copre la generazione client |
| Admin UI / dashboard | Grafana su metriche Prometheus (v2) lo copre meglio |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| GEN-01 | TBD | Pending |
| GEN-02 | TBD | Pending |
| GEN-03 | TBD | Pending |
| GEN-04 | TBD | Pending |
| GEN-05 | TBD | Pending |
| OPS-01 | TBD | Pending |
| OPS-02 | TBD | Pending |
| OPS-03 | TBD | Pending |
| OPS-04 | TBD | Pending |
| OPS-05 | TBD | Pending |
| OPS-06 | TBD | Pending |
| CONF-01 | TBD | Pending |

**Coverage:**
- v1 requirements: 12 total
- Mapped to phases: 0
- Unmapped: 12 ⚠️ (da mappare in roadmap)

---
*Requirements defined: 2026-07-26*
*Last updated: 2026-07-26 after initial definition*
