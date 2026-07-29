# KSUID — K-Sortable Unique IDentifier

**Fonte:** <https://github.com/segmentio/ksuid>
**Autore:** Segment (Twilio)
**Stelle:** 5.3k ⭐
**Linguaggio:** Go (libreria battle-tested)
**Anno:** ~2017

---

## Design

KSUID è stato creato da Segment dopo un'analisi approfondita della storia degli UUID e delle alternative esistenti (vedi ["A Brief History of the UUID"](https://segment.com/blog/a-brief-history-of-the-uuid/)). Combina un timestamp a 32 bit (secondi) con 128 bit di payload casuale per un totale di **160 bit (20 byte)**.

```
 0ujsswThIGTUYm2K8FjOOfXtY1K   (27 caratteri Base62)

|------||---------------------|
 Time        Payload (random)
 32-bit         128-bit
```

## Layout (160 bit totali)

| Campo | Bit | Descrizione |
|---|---|---|
| Timestamp | 32 bit | Unix epoch in secondi, big-endian. Epoch regolato al 13 Maggio 2014 per ~100+ anni di autonomia. |
| Payload | 128 bit | CSPRNG. 2^128 spazio casuale. |

## Proprietà

1. **Naturally ordered by generation time** — ordinabili con UNIX `sort`
2. **Collision-free, coordination-free, dependency-free** — nessun node ID, nessun coordinamento
3. **Highly portable** — rappresentazione Base62 alfanumerica a 27 char

## Rappresentazione

- **Binaria**: 20 byte (32-bit time + 128-bit payload)
- **Testuale**: 27 caratteri Base62, no delimitatori, no caratteri speciali → adatto a URL, filename, log
- Esempio: `0ujsswThIGTUYm2K8FjOOfXtY1K`

## Performance

Libreria Go ottimizzata per zero allocazioni in hot path. Supporta `FastRander` per scenari non security-critical dove la velocità prevale sulla sicurezza crittografica.

API:
- `ksuid.New()` — genera un KSUID
- `ksuid.Parse()` — parsing da stringa
- `ksuid.Sequence` — per generazione concorrente senza contesa di mutex globale

## Battle-tested

In produzione da anni in Segment (ora Twilio). Trilioni di KSUID generati nei sistemi più performance-critical e su larga scala di Segment.

## Confronto KSUID vs Snowflake

| Caratteristica | KSUID | Snowflake |
|---|---|---|
| **Dimensione** | 160 bit (20 byte) | 64 bit (8 byte) |
| **Timestamp precision** | 1 secondo | 1 millisecondo |
| **Clock skew** | Innocuo (128 bit random) | **Critico** |
| **Node ID** | Non richiesto | Richiesto (10 bit) |
| **Collisioni** | Virtualmente impossibile (2^128) | Impossibile (per nodo, clock ok) |
| **Libreria Go** | 5.3k ⭐, battle-tested | Community (bwmarrin/snowflake) |
| **Coordinamento** | Zero assoluto | Node ID assignment |
| **Rappresentazione** | 27 char Base62 | 1-20 char (uint64 string) |
| **Interoperabilità** | Alta (Base62 alfanumerica) | Alta (intero a 64 bit) |

## Punti di forza su Snowflake

1. **Zero coordinamento e zero configurazione** — nessun Node ID
2. **Clock skew irrilevante** — anche con clock che torna indietro, i 128 bit random prevengono duplicati
3. **Non-predictable** — a differenza di Snowflake (dove un avversario potrebbe stimare l'ID successivo), KSUID è crittograficamente imprevedibile
4. **Battle-tested su scala Segment** — trilioni di ID generati in produzione

## Punti deboli

1. **160 bit (20 byte)** — 2.5× la dimensione di Snowflake
2. **Timestamp a secondi, non millisecondi** — ordinamento meno preciso
3. **Non è un intero** — non adatto come primary key integer in database relazionali

## Rilevanza per il progetto

KSUID è l'alternativa più **robusta** in assoluto al problema degli ID distribuiti. Il progetto ha adottato UUIDv7 a 128 bit (MADR 0003) accettando il trade-off dimensionale in favore di robustezza e standardizzazione. KSUID rimane il **gold standard** per sistemi dove la dimensione non è critica.
