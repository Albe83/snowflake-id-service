---
status: accepted
date: 2026-07-29
decision-makers: Albe83
supersedes:
  - 0001-monotonic-anchor-hybrid-sequence
  - 0002-lock-free-generator
---

# ID a 128 bit con payload casuale (stile UUIDv7)

## Context and Problem Statement

Gli ADR 0001 e 0002 hanno introdotto miglioramenti incrementali all'algoritmo Snowflake a 64 bit (monotonic anchor, hybrid sequence, node ID effimero). Tuttavia, ognuno di questi aggiunge complessità per mitigare limiti intrinseci del formato a 64 bit: clock skew, sequence overflow, node ID configuration, prevedibilità.

L'analisi delle alternative moderne (ULID, KSUID, UUIDv7) mostra un trend convergente verso ID a 128 bit con payload casuale, che elimina alla radice tutti questi problemi.

## Decision Drivers

* Semplicità dell'algoritmo
* Eliminazione di configurazione e coordinamento
* Robustezza contro clock skew e restart
* Interoperabilità con standard esistenti (UUIDv7)
* Storage: 16 byte vs 8 byte

## Considered Options

* **64 bit Snowflake modernizzato** (ADR 0001) — monotonic anchor, hybrid sequence, node ID effimero
* **128 bit con payload casuale** — 48 bit Unix timestamp ms + 74 bit random (layout RFC 9562 UUIDv7)

## Decision Outcome

Chosen option: "128 bit con payload casuale", because elimina completamente node ID, clock skew policy, sequence overflow, configurazione e persistenza. L'algoritmo si riduce a due operazioni: leggi il clock, genera random.

Il costo in storage (16 byte vs 8 byte) è considerato trascurabile rispetto ai guadagni in robustezza e semplicità.

### Consequences

* **Good**, because algoritmo ~15 righe di Go, comprensibile in 30 secondi
* **Good**, because zero configurazione, zero coordinamento, zero persistenza
* **Good**, because clock skew innocuo: 74 bit random rendono il duplicato probabilisticamente trascurabile anche con clock che torna indietro
* **Good**, because restart safe senza alcuna precauzione
* **Good**, because nessun mutex: la generazione random può avvenire fuori dal lock
* **Good**, because timestamp Unix epoch standard (interpretabile universalmente)
* **Good**, because 74 bit di spazio casuale per millisecondo (nessun limite di sequence o contatore)
* **Bad**, because 16 byte di storage invece di 8
* **Bad**, because non adatto come primary key integer nativo in database che non supportano UUID nativo

### Confirmation

Verifica tramite test race-enabled:
- Collision test: N goroutine × M ID → zero duplicati (2^74 spazio casuale)
- Clock skew test: `monotonicMs` forzato indietro → zero duplicati
- Restart test: 1000 restart con clock randomizzato → zero duplicati
- Benchmark: throughput di generazione con `crypto/rand`

## Pros and Cons of the Options

### 64 bit Snowflake modernizzato (ADR 0001)

* **Good**, because 8 byte di storage
* **Good**, because intero nativo per database
* **Bad**, because richiede node ID derivation (~10 righe)
* **Bad**, because richiede hybrid sequence logic (~10 righe)
* **Bad**, because richiede mutex per lo stato condiviso
* **Bad**, because clock skew gestito ma non eliminato come concetto
* **Bad**, because 4096 ID/ms/nodo (limite teorico)

### 128 bit con payload casuale

* **Good**, because algoritmo banale (~15 righe)
* **Good**, because nessun concetto di node ID, sequence, skew, overflow
* **Good**, because interoperabile con UUIDv7 (standard IETF)
* **Good**, because throughput illimitato per ms
* **Bad**, because 16 byte di storage (il doppio)
* **Bad**, because richiede `crypto/rand` (dipendenza accettabile)
* **Neutral**, because rappresentazione testuale: 36 caratteri hex vs 1-20 caratteri decimali

## More Information

- Design document: `docs/design/uuidv7-generator-workflow.md`
- Riferimento standard: RFC 9562 (UUIDv7)
- Questo ADR sostituisce 0001 e 0002 — i miglioramenti lì descritti sono assorbiti dal design a 128 bit
