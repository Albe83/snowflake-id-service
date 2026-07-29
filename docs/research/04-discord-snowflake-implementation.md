# Discord: Snowflake API Reference

**Fonte:** https://discord.com/developers/docs/reference
**Tipo:** Documentazione API ufficiale
**Ultimo accesso:** 2026-07-29

## Utilizzo in Discord

Discord utilizza il formato Snowflake di Twitter per tutti gli ID univoci nel sistema. Gli ID sono garantiti univoci attraverso tutto Discord (con rare eccezioni per oggetti figli che condividono l'ID del padre).

## Struttura Snowflake di Discord

| Campo | Bit | # Bit | Descrizione | Decode |
|-------|-----|-------|-------------|--------|
| Timestamp | 63-22 | 42 bit | Millisecondi dal Discord Epoch (1420070400000) | `(snowflake >> 22) + 1420070400000` |
| Internal worker ID | 21-17 | 5 bit | | `(snowflake & 0x3E0000) >> 17` |
| Internal process ID | 16-12 | 5 bit | | `(snowflake & 0x1F000) >> 12` |
| Increment | 11-0 | 12 bit | Contatore per-processo | `snowflake & 0xFFF` |

## Dettagli Chiave

- **Epoch custom**: `1420070400000` (primo secondo del 2015)
- **42 bit di timestamp** (invece dei 41 standard): Discord usa 42 bit, sufficienti per ~139 anni dall'epoch
- **Worker ID + Process ID**: 5 bit ciascuno, splitting dei 10 bit machine ID in due campi separati
- **Serializzazione**: sempre **stringa** in HTTP API per prevenire overflow in linguaggi senza uint64 nativo
- **Paginazione**: gli Snowflake sono usati per `before`/`after` nella paginazione delle API
- **Generazione da timestamp**: `(timestamp_ms - DISCORD_EPOCH) << 22` per creare un ID corrispondente a un momento specifico

## Esempio

ID: `175928847299117063`

Componenti:
- Timestamp: `41944705796` + `1420070400000` = Unix ms `1462015105796`
- Data: `2016-04-30 11:18:25.796 UTC`

## Rilevanza per il Progetto

- Dimostra l'uso di 42 bit per il timestamp (invece di 41) — flessibilità dell'architettura
- Conferma lo split del machine ID in worker + process come pattern comune
- La generazione da timestamp (`<< 22`) è un'operazione standard per la paginazione
- La serializzazione come stringa è confermata anche da Discord su scala enorme
- Conferma il pattern epoch custom per organizzazione
