# ULID — Universally Unique Lexicographically Sortable Identifier

**Fonte:** <https://github.com/ulid/spec>
**Stelle:** 10.8k ⭐
**Tipo:** Specifica indipendente, nativa Go (`oklog/ulid`)
**Anno:** ~2016

---

## Design

ULID è stato progettato per risolvere le criticità di UUID (troppo lungo, non sortable) e di Snowflake (clock skew, node ID).

```
 01AN4Z07BY      79KA1307SR9X4MV3

|----------|    |----------------|
 Timestamp          Randomness
   48bit             80bit
```

## Layout binario (128 bit / 16 byte)

| Campo | Bit | Descrizione |
|---|---|---|
| Timestamp | 48 bit | Unix-time in millisecondi. Fino all'anno 10889 AD. |
| Randomness | 80 bit | CSPRNG (cryptographically secure). 1.21e+24 ULID/ms unici. |

## Rappresentazione

- Formato **Crockford's Base32** (26 caratteri, case-insensitive, URL-safe, niente I/L/O/U)
- Esempio: `01ARZ3NDEKTSV4RRFFQ69G5FAV`
- vs UUID: 26 caratteri vs 36 caratteri (UUID)

## Monotonicità

Quando più ULID sono generati nello stesso millisecondo, il componente `random` viene incrementato (1 bit in LSB con carry). Esempio:

```
01BX5ZZKBKACTAV9WEVGEMMVRZ  // prima chiamata
01BX5ZZKBKACTAV9WEVGEMMVS0  // seconda chiamata (stesso ms)
```

Se vengono generati più di 2^80 ULID nello stesso millisecondo (praticamente impossibile), la generazione lancia errore.

## Confronto ULID vs Snowflake

| Caratteristica | ULID | Snowflake |
|---|---|---|
| **Dimensione** | 128 bit (16 byte) | 64 bit (8 byte) |
| **Node ID** | Non richiesto | 10 bit (1024 nodi) |
| **Clock skew** | Innocuo (random 80 bit prevengono duplicati) | Richiede policy esplicita |
| **Sequence bottleneck** | 2^80/ms (impossibile) | 4096/ms |
| **Timestamp** | Unix epoch standard | Epoch custom |
| **Rappresentazione** | 26 char Base32 | Fino a 20 char (uint64 string) |
| **Coordinamento** | Zero | Node ID assignment |
| **Standard** | Community spec | De facto (Twitter) |

## Punti di forza su Snowflake

1. **Nessun node ID da configurare** — elimina complessità operativa
2. **Clock skew non è un problema** — 80 bit di casualità garantiscono l'unicità anche se il clock torna indietro
3. **Standard de facto per UUID alternativi** — adozione molto ampia (10.8k stelle, port in decine di linguaggi)
4. **Rappresentazione compatta** — 26 caratteri Base32 vs 36 dello UUID standard
5. **Timestamp standard** — Unix epoch, non custom

## Punti deboli

1. **128 bit** — il doppio di Snowflake per storage
2. **Probabilistico** — tecnicamente possibile (anche se ~impossibile) una collisione
3. **Non leggibile come intero** — la rappresentazione Base32 non è adatta a database che si aspettano integer primary key

## Rilevanza per il progetto

ULID è l'alternativa più matura a Snowflake per chi vuole **azzerare la complessità operativa** (niente node ID, niente clock skew policy) ed è disposto a pagare il costo di 128 bit. Il progetto ha adottato UUIDv7 (MADR 0003) che condivide lo stesso principio di design a 128 bit con payload casuale, aggiungendo il vantaggio di uno standard IETF.
