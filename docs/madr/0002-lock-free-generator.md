---
status: superseded
date: 2026-07-29
decision-makers: Albe83
superseded-by:
  - 0003-128-bit-random-payload
---

> **Superato da:** ADR 0003 — il modello UUIDv7 stateless non ha sezioni critiche da proteggere. Nessun mutex, nessun atomic-CAS necessario.

# Lock-Free Generator via `atomic.CompareAndSwap`

## Context and Problem Statement

L'attuale generatore usa `sync.Mutex` per serializzare l'accesso allo stato (`monotonicMs`, `seqCounter`, `seqRandom`). Sotto carico concorrente, le goroutine fanno coda sul mutex.

Alternative moderne come XID (`rs/xid`) sono lock-free e mostrano benchmark di ~32 ns/op, mentre implementazioni con lock globale (UUIDv1) degradano all'aumentare dei core.

La domanda: il guadagno in throughput giustifica la complessità aggiuntiva di un'implementazione lock-free?

## Decision Drivers

* Semplicità e manutenibilità del codice
* Throughput sotto carico concorrente
* Testabilità e correttezza (race detector)
* Volumi attesi: bassi (il servizio è usato internamente al framework Foundation)

## Considered Options

* **`sync.Mutex`** (status quo)
* **`atomic.CompareAndSwap`** (lock-free)

## Decision Outcome

_Status: proposed — in attesa di verifica con benchmark su carico reale._

La decisione preliminare propende per mantenere `sync.Mutex` almeno fino a quando i volumi reali non giustificano il passaggio a lock-free. Il mutex è più semplice da implementare, testare e verificare con il race detector.

### Consequences

* **Good**, because codice più semplice e meno soggetto a bug
* **Good**, because il race detector di Go verifica la correttezza del mutex in modo trasparente
* **Bad**, because throughput limitato a ~1M ID/s per core sotto contesa
* **Bad**, because possibile coda di goroutine su burst di richieste

### Confirmation

Da definire: benchmark con `go test -bench` simulando carico concorrente (N goroutine × M ID) per determinare il punto di saturazione del mutex.

## Pros and Cons of the Options

### `sync.Mutex`

* **Good**, because implementazione banale (~5 righe)
* **Good**, because il race detector segnala automaticamente accessi non protetti
* **Good**, because comportamento deterministico e prevedibile
* **Bad**, because serializza tutte le goroutine su un singolo lock
* **Bad**, because non scala con l'aumentare dei core

### `atomic.CompareAndSwap`

* **Good**, because throughput più alto (XID: ~32 ns/op)
* **Good**, because scala linearmente col numero di core
* **Bad**, because richiede logica di retry su CAS fallito
* **Bad**, because più difficile da testare (il race detector non copre tutte le race condition su atomici)
* **Bad**, because complessità implementativa (~30 righe vs ~5)
* **Neutral**, because per volumi bassi il guadagno è impercettibile

## More Information

- Riferimento: XID (`rs/xid`) — generatore lock-free in Go, ~4.3k stelle
- Benchmark pertinenti: eseguire `go test -bench . -cpu 1,2,4,8` per misurare la scalabilità del mutex sotto carico
- Il passaggio a lock-free è retrocompatibile (l'interfaccia `NextID() (int64, error)` non cambia)
