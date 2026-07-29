# Sonyflake — Sony's Snowflake Variant

**Fonte:** <https://github.com/sony/sonyflake>
**Autore:** Sony
**Stelle:** 4.4k ⭐
**Linguaggio:** Go
**Tipo:** Variante diretta di Snowflake con diversa allocazione dei bit

---

## Design

Sonyflake è un generatore di ID distribuiti **direttamente ispirato a Snowflake**, ma con un layout di bit diverso ottimizzato per:
- **Lifetime più lungo** (174 anni vs 69)
- **Più istanze distribuite** (2^16 = 65536 vs 2^10 = 1024)
- **Time unit a 10ms** invece di 1ms

## Layout (63 bit variabili + 1 bit segno = 64 bit)

| Campo | Bit | Descrizione |
|---|---|---|
| Timestamp | 39 bit | Unità di 10 millisecondi da StartTime |
| Sequence | 8 bit | 256 ID per 10ms per macchina |
| Machine ID | 16 bit | 65536 istanze possibili |

Default StartTime: `2025-01-01 00:00:00 UTC`

## Configurabilità

A differenza di Snowflake (layout fisso), Sonyflake permette di **personalizzare**:
- `BitsSequence` (default 8)
- `BitsMachineID` (default 16)
- `TimeUnit` (default 10ms, minimo 1ms)
- `StartTime` (default 2025-01-01)
- `MachineID` — funzione custom per ottenere l'ID macchina
- `CheckMachineID` — validazione custom

Il bit length del tempo è calcolato come `63 - BitsSequence - BitsMachineID`. Deve essere ≥ 32.

## Machine ID automatico

Di default, Sonyflake usa i **16 bit bassi dell'IP privato** della macchina. Su AWS/VPC, l'IP privato è unico per istanza → machine ID automatico e distribuito. Pacchetto `awsutil` per AmazonEC2.

## Confronto Sonyflake vs Snowflake

| Caratteristica | Sonyflake | Snowflake |
|---|---|---|
| **Timestamp bit** | 39 bit (10ms unit) | 41 bit (1ms unit) |
| **Lifetime** | ~174 anni | ~69 anni |
| **Machine ID bit** | 16 bit (65536 nodi) | 10 bit (1024 nodi) |
| **Sequence bit** | 8 bit (256/10ms) | 12 bit (4096/ms) |
| **Max ID rate** | 25600/s per nodo | 4096000/s per nodo |
| **Time unit** | 10ms | 1ms |
| **Configurabilità** | Alta (bit, unit, epoch) | Fissa (solo epoch) |
| **Machine ID** | Auto (da IP) | Manuale / Zookeeper |

## Trade-off di Sonyflake

### Vantaggi
1. **Più nodi** (65536 vs 1024) — meglio per deployment su larga scala
2. **Lifetime più lungo** — 174 anni vs 69 anni
3. **Configurabile** — adattabile a esigenze specifiche
4. **Machine ID automatico** — nessuna configurazione manuale su AWS/cloud

### Svantaggi
1. **Molti meno ID al secondo** — 25.6K/s vs 4M/s (fattore 156×)
2. **Time unit più grossolana** — 10ms anziché 1ms (ordinamento meno preciso)
3. **Ancora vulnerabile al clock skew** — come Snowflake, richiede clock monotono
4. **Machine ID da IP** — non funziona in reti dove gli IP non sono unici o stabili

## Rilevanza per il progetto

Sonyflake dimostra che il layout Snowflake **non è l'unica allocazione possibile**. Il trade-off tra numero di nodi e throughput è flessibile. Il progetto ha adottato UUIDv7 (MADR 0003) che non è vincolato a nessun layout Snowflake-derivato: 48 bit timestamp + 74 bit random, senza node ID né sequence.
