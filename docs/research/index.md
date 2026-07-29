# Snowflake ID — Ricerca e Analisi

## Indice dei Documenti

| File | Contenuto | Tipo | Anno |
|------|-----------|------|------|
| [01-twitter-announcing-snowflake.md](01-twitter-announcing-snowflake.md) | Annuncio originale di Snowflake su Twitter Engineering Blog | Fonte primaria | 2010 |
| [02-wikipedia-snowflake-id.md](02-wikipedia-snowflake-id.md) | Voce enciclopedica su Snowflake ID | Riferimento generale | 2026 |
| [03-twitter-github-reference.md](03-twitter-github-reference.md) | Repository archiviato con implementazione di riferimento (Scala) | Codice sorgente | 2010 |
| [04-discord-snowflake-implementation.md](04-discord-snowflake-implementation.md) | Documentazione API Discord sull'uso di Snowflake | Implementazione reale | — |
| [05-instagram-sharding-ids.md](05-instagram-sharding-ids.md) | Approccio modificato di Instagram per sharding PostgreSQL | Implementazione reale | 2016 |
| [06-flickr-ticket-servers.md](06-flickr-ticket-servers.md) | Approccio alternativo di Flickr (ticket server MySQL) | Alternativa | 2010 |
| [07-rfc9562-uuidv7.md](07-rfc9562-uuidv7.md) | Standard IETF UUIDv7 come successore degli ID time-ordered | Standard | 2024 |
| [08-segment-ksuid-critique.md](08-segment-ksuid-critique.md) | KSUID e critica agli approcci Snowflake-like | Critica | 2017 |
| [09-critical-analysis.md](09-critical-analysis.md) | Analisi critica sintetica dell'approccio Snowflake | Analisi | — |

## Fonti Consultate

### Fonti Primarie (autorevoli)
1. **Twitter Engineering Blog** — Announcing Snowflake (2010). URL archiviato su Internet Archive.
2. **GitHub twitter-archive/snowflake** — Implementazione di riferimento (archiviata), ~7.8k stelle.
3. **Discord Developer Portal** — API Reference, sezione Snowflakes.
4. **Instagram Engineering** — Sharding & IDs at Instagram (2016).
5. **Flickr Engineering (code.flickr.net)** — Ticket Servers (2010).

### Standard e Specifiche
6. **RFC 9562** — Universally Unique IDentifiers (UUIDs), IETF Proposed Standard (Maggio 2024).
7. **Wikipedia** — Snowflake ID, voce enciclopedica aggiornata a Luglio 2026.

### Analisi Critiche
8. **Segment (Twilio) Engineering Blog** — A Brief History of the UUID (2017), analisi che introduce KSUID con critiche esplicite all'approccio Snowflake.

## Sintesi dei Risultati

### Come funziona Snowflake ID
Un intero a 64 bit composto da:
- **Timestamp** (41 bit): millisecondi da un epoch custom
- **Machine ID** (10 bit): identificatore univoco del generatore
- **Sequence number** (12 bit): contatore che resetta ogni millisecondo

**Proprietà**: ordinabile temporalmente, unico per macchina, zero dipendenze esterne, generabile in-memory.

### Chi lo usa (varianti reali)
- **Twitter/X**: 41/10/12, epoch 1288834974657
- **Discord**: 42/5+5/12, epoch 1420070400000
- **Instagram**: 41/13/10, epoch custom

### Critiche principali
1. **Clock skew**: se l'orologio torna indietro, rischio di ID duplicati. Richiede policy esplicita.
2. **Node ID**: 10 bit limitano a 1024 generatori; richiede coordinamento out-of-band.
3. **Sequence bottleneck**: 4096 ID/ms per macchina può essere insufficiente per carichi estremi.
4. **Mancanza di standard**: ogni implementazione ha layout, epoch e policy diverse.
5. **Non estendibile**: formato a 64 bit chiuso, non si possono aggiungere bit.

### Alternativa standard: UUIDv7 (RFC 9562, 2024)
- 128 bit: 48 bit timestamp Unix + 74 bit random
- Zero configurazione, zero clock skew issues, zero collisioni
- Svantaggio: il doppio dello spazio (16 vs 8 byte), non adatto se lo storage è critico

### Alternativa ultra-robusta: KSUID (Segment, 2017)
- 160 bit: 32 bit timestamp + 128 bit random payload
- Formato human-friendly Base62 a 27 caratteri
- Clock skew innocuo per design

### Verdetto per il nostro progetto
Il progetto ha adottato UUIDv7 a 128 bit conforme a RFC 9562 (MADR 0003), che elimina completamente i problemi di clock skew, node ID, sequence overflow e configurazione. Il costo in storage (16 vs 8 byte) è stato considerato trascurabile rispetto ai guadagni in robustezza, semplicità e interoperabilità standard.
