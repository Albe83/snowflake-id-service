# Wikipedia: Snowflake ID

**Fonte:** https://en.wikipedia.org/wiki/Snowflake_ID
**Tipo:** Enciclopedia, voce di riferimento
**Ultimo aggiornamento:** 2 Luglio 2026

## Definizione

Snowflake ID (o "snowflakes") sono identificatori univoci a 64 bit usati nel calcolo distribuito. Creati da X (Twitter) per i tweet. Il nome deriva dalla credenza che ogni fiocco di neve abbia una struttura unica.

## Formato Standard

Layout a 64 bit:

| Bit | Campo | Descrizione |
|-----|-------|-------------|
| 63 | Riservato | Sempre 0 (garantisce numero positivo signed) |
| 62-22 | Timestamp | 41 bit, millisecondi dall'epoch custom |
| 21-12 | Machine ID | 10 bit, previene collisioni tra macchine |
| 11-0 | Sequence | 12 bit, contatore per-macchina per millisecondo |

**Solo 63 bit sono variabili** — il bit più alto è sempre 0 per compatibilità con interi signed a 64 bit. Serializzato come stringa decimale in JSON (non number, per evitare overflow in JS).

Proprietà: **sortable by time** — gli ID sono ordinabili temporalmente e il timestamp può essere estratto da un ID.

## Implementazioni Note

| Organizzazione | Varianti |
|----------------|----------|
| **Twitter/X** | 5-bit datacenter ID + 5-bit worker ID (splittando i 10 bit machine ID). Epoch: `1288834974657` |
| **Discord** | 5-bit internal worker ID + 5-bit internal process ID + 12-bit increment. Epoch: `1420070400000` (1 Gen 2015) |
| **Instagram** | 41-bit timestamp + 13-bit shard ID + 10-bit sequence. Layout modificato |
| **Mastodon** | 48-bit timestamp UNIX epoch + 16-bit sequence. Layout semplificato |

## Decode Esempio

Tweet @Wikipedia (Feb 2025):
- ID: `1888944671579078978`
- Binary: `0001 1010 0011 0110 1110 0001 0010 1011 1011 0101 11 | 01 0110 1000 | 0001 0100 0010`
- Timestamp: `450359504599` + Epoch X `1288834974657` = Unix ms `1739194479256` (10 Feb 2025 13:34:39.256 UTC)
- Machine ID: `01 0110 1000`
- Sequence: `322` (322° ID in quel millisecondo)

## Rilevanza per il Progetto

- Conferma il layout standard 41/10/12 come riferimento de facto
- Mostra le varianti reali: Discord splitta i 10 bit, Instagram cambia layout, Mastodon semplifica
- La serializzazione come **stringa** (non number) è una best practice verificata cross-implementazione
- L'epoch è custom per ogni organizzazione, confermando la pratica del nostro progetto
