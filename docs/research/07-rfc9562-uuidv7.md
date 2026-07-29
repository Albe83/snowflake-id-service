# RFC 9562: UUID Version 7 — Successore Standard degli ID Time-Ordered (2024)

**Fonte:** https://datatracker.ietf.org/doc/html/rfc9562
**Data:** Maggio 2024 (obsoleta RFC 4122)
**Tipo:** Standard IETF (Proposed Standard)

## Contesto

L'IETF ha aggiornato lo standard UUID dopo 20 anni (RFC 4122 era del 2005). Ha analizzato **16 implementazioni proprietarie di ID time-ordered** (Snowflake, ULID, KSUID, Sonyflake, XID, ecc.) e ha standardizzato una soluzione comune: UUIDv7.

## Problemi Identificati che Snowflake Cerca di Risolvere

1. **UUIDv4 Locality**: insert casuali in database B-tree → frammentazione e degrado delle performance
2. **Timestamp non standard in UUIDv1**: Gregorian epoch a 100ns, non rappresentabile in IEEE 754
3. **MAC address in UUIDv1**: esposizione di informazioni sulla macchina, problemi di privacy
4. **Ordinamento non banale**: UUIDv1 richiede parsing, non ordinamento byte-by-byte

**Snowflake era una delle 16 soluzioni analizzate** per disegnare UUIDv7.

## UUIDv7: Layout

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                        unix_ts_ms (48 bit)                    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  unix_ts_ms  | ver=7 |          rand_a / counter (12 bit)     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|var=10|               rand_b / counter (62 bit)                |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                        rand_b / counter                       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Totale**: 48 bit timestamp + 4 bit version + 2 bit variant + **74 bit random counter** = 128 bit

## UUIDv7 vs Snowflake: Confronto Critico

| Dimensione | UUIDv7 (128 bit) | Snowflake (64 bit) |
|-----------|------------------|---------------------|
| **Numero di macchine** | Illimitato (74 bit casuali) | 1024 (10 bit) |
| **ID per ms per macchina** | Illimitato (contatore 74 bit) | 4096 (12 bit, spin-wait se esaurito) |
| **Clock skew** | Non rilevante (contatore monotono + casualità) | **Critico**: richiede policy esplicita |
| **Standard** | IETF RFC 9562 | Nessuno (de facto) |
| **Timestamp** | Unix epoch standard (48 bit ms) | Epoch custom (non interoperabile) |
| **Dimensione storage** | 16 byte | 8 byte |
| **Librerie native** | In sviluppo in tutti i linguaggi | Non standard, librerie community |
| **Collisione** | Probabilistica (2^-74 ≈ 5.3×10^-23) | **Garantita impossibile** (per macchina) |
| **Sicurezza/Privacy** | Nessuna info host nel ID | Nessuna info host (10 bit anonimi) |

## Implicazioni per Snowflake

### Vantaggi di UUIDv7 su Snowflake
1. **Nessuna configurazione**: nessun node ID da assegnare/configurare/manutenere
2. **Nessun limite di scala**: può scalare a qualsiasi numero di generatori senza riallocare bit
3. **Clock skew irrilevante**: anche se il clock torna indietro, casualità e contatore prevengono collisioni
4. **Standard IETF**: interoperabilità con qualsiasi sistema che supporta UUID
5. **74 bit di casualità**: collisioni virtualmente impossibili

### Vantaggi di Snowflake su UUIDv7
1. **Dimensione**: 8 byte vs 16 byte (metà dello spazio)
2. **Zero probabilità di collisione**: la sequence è deterministica, non probabilistica
3. **Più semplice da implementare**: ~100 linee di codice, nessun CSPRNG richiesto
4. **Prevedibile/leggibile**: l'ID contiene informazioni semantiche (timestamp, node)
5. **Migliore per indici database**: 64 bit integer vs 128 bit, minor overhead di storage

## Rilevanza per il Nostro Progetto

- UUIDv7 è il competitor standard più diretto di Snowflake
- La scelta di **64 bit vs 128 bit** è il trade-off principale
- Per un sistema a **volumi bassi e numero di nodi contenuto** (come il nostro), i limiti di Snowflake (1024 macchine, 4096 ID/ms) non sono un problema reale
- L'IETF ha scelto un approccio probabilistico (74 bit casuali) per **eliminare la complessità di configurazione del node ID**, non perché Snowflake sia tecnicamente inferiore
- Snowflake rimane superiore quando lo spazio di storage è critico e il numero di generatori è noto e limitato
