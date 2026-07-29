# XID — Globally Unique ID Generator (MongoDB-inspired)

**Fonte:** <https://github.com/rs/xid>
**Autore:** Olivier Poitrey (rs)
**Stelle:** 4.3k ⭐
**Linguaggio:** Go (libreria)
**Tipo:** Globally unique ID basato su algoritmo MongoDB ObjectID

---

## Design

XID adotta l'algoritmo di MongoDB ObjectID con una diversa serializzazione (Base32hex) per compattezza. Si posiziona **tra Snowflake (64 bit) e UUID/KSUID (128-160 bit)** con i suoi **96 bit (12 byte)**.

```
 9m4e2mr0ui3e8a215n4g    (20 caratteri Base32hex)

|------||-----||--||-----|
  Time   Machine PID  Counter
  4B      3B     2B   3B
```

## Layout (96 bit / 12 byte)

| Campo | Dimensione | Descrizione |
|---|---|---|
| Timestamp | 4 byte (32 bit) | Secondi dall'Unix epoch |
| Machine ID | 3 byte (24 bit) | Identificatore della macchina (default: hostname hash) |
| Process ID | 2 byte (16 bit) | PID del processo |
| Counter | 3 byte (24 bit) | Contatore inizializzato con valore random |

Totale: **16.777.216 ID unici per secondo per host/process**.

## Rappresentazione

- **Base32hex** (RFC 4648): 20 caratteri, lowercase `[0-9a-v]`, sempre sortable
- Esempio: `9m4e2mr0ui3e8a215n4g`
- **Compatibile con MongoDB ObjectID** (12 byte binari), ma stringa più corta (20 vs 24 char hex)

## Confronto dimensionale

| Sistema | Dimensione binaria | Rappresentazione stringa |
|---|---|---|
| Snowflake | 8 byte | fino a 20 char |
| **XID** | **12 byte** | **20 char** |
| MongoDB ObjectID | 12 byte | 24 char (hex) |
| UUID | 16 byte | 36 char |
| KSUID | 20 byte | 27 char |
| ULID | 16 byte | 26 char |

## Caratteristiche distintive

1. **Lock-free** — nessun mutex nella generazione (a differenza di Snowflake)
2. **Machine ID automatico** — derivato dall'hostname, configurabile via env `XID_MACHINE_ID`
3. **PID incluso** — garantisce unicità tra processi sulla stessa macchina senza coordinamento
4. **Counter inizializzato random** — anche dopo restart, improbabile collisione su stesso timestamp
5. **Precisione 1 secondo** (non millisecondi) — 24 bit di counter compensano

## Confronto XID vs Snowflake

| Caratteristica | XID | Snowflake |
|---|---|---|
| **Dimensione** | 96 bit (12 byte) | 64 bit (8 byte) |
| **Timestamp** | 32 bit, 1s precision | 41 bit, 1ms precision |
| **Counter/Sequence** | 24 bit (16.7M/s) | 12 bit (4k/ms ≈ 4M/s) |
| **Machine ID** | 24 bit (automatico) | 10 bit (da configurare) |
| **Lock** | Lock-free | Mutex |
| **Clock skew** | Tollerante (counter random) | Richiede policy |
| **Coordinamento** | PID + Machine ID auto | Node ID manuale |
| **Interoperabilità DB** | 12 byte (compatibile MongoDB) | 8 byte (int64) |

## Punti di forza su Snowflake

1. **Lock-free** — non ha contesa di mutex sotto carico
2. **Machine ID automatico** — nessuna configurazione manuale
3. **Counter a 24 bit** — throughput molto più alto (16.7M vs 4M ID/s)
4. **Più robusto al clock skew** — counter randomizzato all'avvio + timestamp a granularità 1s
5. **Compatibilità MongoDB** — lo stesso formato binario di ObjectID

## Punti deboli

1. **96 bit (12 byte)** — 1.5× la dimensione di Snowflake
2. **Timestamp a secondi** — ordinamento meno preciso
3. **PID esposto** — potenziale information leak (process ID della macchina)
4. **Machine ID da hostname** — collisioni possibili in container con stesso hostname

## Rilevanza per il progetto

XID rappresenta un **compromesso dimensionale interessante** (12 byte) tra Snowflake (8) e UUID/KSUID (16-20). Il design lock-free è superiore per throughput. Per il nostro caso d'uso, 96 bit sono comunque più dei 64 bit richiesti.
