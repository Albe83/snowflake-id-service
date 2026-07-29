---
status: superseded
date: 2026-07-29
decision-makers: Albe83
superseded-by:
  - 0002-lock-free-generator
  - 0003-128-bit-random-payload
---

# Monotonic Anchor + Ephemeral Node ID + Hybrid Sequence per la generazione Snowflake ID

## Context and Problem Statement

L'algoritmo Snowflake originale (Twitter, 2010) ha tre punti deboli noti:
1. **Clock skew**: se l'orologio di sistema torna indietro, il generatore restituisce errore, causando potenziali outage a cascata.
2. **ID prevedibili**: la sequence puramente incrementale (0, 1, 2, …) permette a un avversario di stimare l'ID successivo.
3. **Node ID statico cross-restart**: se il clock arretra tra un restart e l'altro, il generatore può produrre duplicati senza accorgersene.

Fonti: Twitter reference implementation, Segment KSUID, RFC 9562 UUIDv7, ULID.

## Considered Options

* **Snowflake originale**: reject su clock skew, sequence incrementale pura, node ID statico
* **Monotonic anchor + ephemeral node ID + hybrid sequence**: contatore monotono interno, node ID derivato a ogni restart, sequence splittata in counter + random

## Decision Outcome

Chosen option: "Monotonic anchor + ephemeral node ID + hybrid sequence"

### Consequences

* **Good**, because elimina la classe di errore più frequente in produzione (clock skew → 503)
* **Good**, because gli ID non sono più predicibili da un avversario
* **Good**, because nessuna necessità di persistenza tra restart (node ID cambia a ogni ciclo di vita)
* **Good**, because zero configurazione manuale del node ID
* **Bad**, because il node ID effimero consuma più ID nello spazio a 10 bit — irrilevante con 1024 slot disponibili
* **Bad**, because gli ID generati durante uno skew hanno timestamp leggermente "vecchio" — skew tipici < 5ms, trascurabile

### Confirmation

Verifica tramite test race-enabled:
- Test di skew simulato: `monotonicMs` non arretra mai
- Test di restart: node ID diverso a ogni `NewGenerator()`
- Test di predicibilità: ID consecutivi nello stesso ms condividono i bit random, ID di ms diversi hanno pattern casuali diversi

## Pros and Cons of the Options

### Snowflake originale

* **Good**, because implementazione di riferimento consolidata
* **Bad**, because il clock skew causa errori a runtime
* **Bad**, because ID predicibili (sequence incrementale)
* **Bad**, because richiede persistenza o coordinamento per node ID cross-restart

### Monotonic anchor + ephemeral node ID + hybrid sequence

* **Good**, because `NextID()` non restituisce mai errore
* **Good**, because nessuna configurazione manuale
* **Good**, because nessuna dipendenza da file system
* **Neutral**, because aggiunge ~10 righe di logica rispetto all'originale
* **Bad**, because introduce `crypto/rand` per i bit random della sequence (dipendenza accettabile)

## More Information

- Design document: `docs/design/uuidv7-generator-workflow.md`
- La sequence ibrida alloca 6 bit al contatore incrementale e 6 bit random. L'allocazione è configurabile.
- Il node ID è derivato come `PID ⊕ (startupEpoch & 0x3FF) ⊕ rand() & 0x3FF`.
