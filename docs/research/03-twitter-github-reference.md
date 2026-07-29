# Twitter: Reference Implementation (GitHub Archive)

**Fonte:** https://github.com/twitter-archive/snowflake
**Tipo:** Repository archiviato, implementazione di riferimento (read-only dal 2021)
**Linguaggio:** Scala
**Tag rilevante:** `snowflake-2010`

## Stato del Repository

Archiviato da Twitter il 18 Settembre 2021. Read-only. Il README indica che la versione iniziale (2010) è stata ritirata e Twitter stava lavorando a una riscrittura basata su Twitter-server (Finagle), mai pubblicata.

## Struttura dell'Implementazione (snowflake-2010)

Il file chiave è `IdWorker.scala`:

```
IdWorker.scala#L27: workerIdBits = 5, datacenterIdBits = 5
```

Il machine ID a 10 bit è splittato in:
- **5 bit datacenter ID**
- **5 bit worker ID**

Questo permette 32 datacenter × 32 worker = 1024 combinazioni totali, equivalente a 10 bit pieni.

## Caratteristiche Tecniche

- Sequence number per-worker (non per-thread come descritto nel blog)
- Mutex-guarded: usa `synchronized` per thread safety
- Spin-wait su sequence overflow: se la sequence esaurisce in un millisecondo, attende il millisecondo successivo
- Epoch: `1288834974657` (Twitter custom epoch)
- Il timestamp è validato: genera eccezione se l'orologio va indietro (`clock moved backwards`)

## Rilevanza per il Progetto

- **Fonte primaria per il design dell'IdWorker**: mutex, spin-wait, validazione clock
- Il pattern `synchronized` in Scala equivale al nostro `sync.Mutex` in Go
- La validazione del clock che torna indietro è il comportamento di default, non una policy configurabile — il nostro progetto deve rendere questa policy esplicita
- L'epoch custom è un pattern confermato da Twitter stesso
