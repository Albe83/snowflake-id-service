# Twitter: Announcing Snowflake (2010)

**Fonte:** https://web.archive.org/web/20190301065041/https://blog.twitter.com/engineering/en_us/a/2010/announcing-snowflake.html
**Autore:** Ryan King (@rk), Twitter Engineering
**Data:** 1 Giugno 2010
**Tipo:** Blog post ufficiale, annuncio originale

## Contesto

Twitter stava migrando da MySQL a Cassandra e sharded MySQL (via Gizzard). MySQL ha auto-increment built-in, ma Cassandra e MySQL sharded no — serviva un sistema di generazione ID distribuito senza coordinamento.

## Il Problema

Tre requisiti stringenti:
1. **Decine di migliaia di ID al secondo** in alta disponibilità → approccio non coordinato
2. **Roughly sortable**: tweet A e B pubblicati nello stesso periodo devono avere ID vicini. Obiettivo: k-sorted con k < 1 secondo
3. **64 bit**: Twitter aveva già sofferto l'espansione dei bit per i tweet ID (Twitpocalypse) con oltre 100.000 codebase da aggiornare

## Opzioni Scartate

| Opzione | Motivo rifiuto |
|---------|---------------|
| MySQL ticket server (stile Flickr) | Non garantiva ordinamento senza routine di re-sync |
| UUID | Tutti gli schemi noti richiedevano 128 bit |
| Zookeeper sequential nodes | Performance insufficienti; approccio coordinato abbassa la disponibilità |

## Soluzione

Composizione di tre campi a 64 bit:
- **Timestamp** (41 bit) — millisecondi dall'epoch
- **Worker number** (10 bit) — scelto via Zookeeper all'avvio, sovrascrivibile via config
- **Sequence number** (12 bit) — per-thread

I worker number sono assegnati via Zookeeper (con fallback a config file), ma il meccanismo di generazione è **uncoordinated** — ogni worker genera autonomamente.

## Dettagli Tecnici Chiave

- Sequence number è **per-thread**, non per-processo
- Worker number da Zookeeper, ma il meccanismo di startup non è bloccante (config file fallback)
- Codice open source su GitHub, software alpha-quality, non ancora in produzione al momento dell'annuncio
- Garantisce **k-sorted** con k < 1 secondo

## Rilevanza per il Progetto

- **Fonte canonica primaria** dell'algoritmo Snowflake
- Dettaglio importante: Twitter usava sequence **per-thread**, non per-processo come molte implementazioni successive
- L'uso di Zookeeper per worker ID è un compromesso: fornisce un registro centralizzato per l'assegnazione, ma il generatore stesso è non coordinato
