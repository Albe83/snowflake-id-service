# Instagram: Sharding & IDs

**Fonte:** https://instagram-engineering.com/sharding-ids-at-instagram-1cf5a71e5a5c
**Autore:** Instagram Engineering
**Data:** 2 Maggio 2016
**Tipo:** Engineering blog post

## Contesto

Instagram aveva bisogno di shardare i dati su migliaia di server PostgreSQL. Serviva un sistema di generazione ID che funzionasse con il partitioning logico senza richiedere migration complesse quando si aggiungevano nuovi shard.

## Approccio: Snowflake Modificato

Instagram ha adottato una versione modificata del formato Snowflake:
- **41 bit timestamp** (millisecondi dall'epoch custom)
- **13 bit shard ID** (invece dei 10 bit standard)
- **10 bit sequence** (invece dei 12 bit standard)

Layout: `timestamp(41) | shard_id(13) | sequence(10)`

## Motivazione delle Modifiche

1. **13 bit per shard ID** (8192 shard): Instagram opera con migliaia di shard PostgreSQL, quindi aveva bisogno di più bit per identificare lo shard
2. **10 bit per sequence** (1024 ID/ms per shard): sufficiente per i volumi di Instagram per shard

## Dettagli Implementativi

- **Generazione in-app**: gli ID sono generati direttamente nell'applicazione (non in un servizio separato)
- **Shard ID da schema PostgreSQL**: ogni shard logico ha un ID preassegnato, derivato dallo schema PostgreSQL
- **Nessun coordinamento**: gli shard ID sono statici e preconfigurati
- L'ID generato contiene già l'informazione dello shard → routing trasparente

## Rilevanza per il Progetto

- Dimostra la **flessibilità del layout Snowflake**: bit allocation adattabile a esigenze specifiche
- **Generazione in-app vs servizio separato**: Instagram genera ID nell'applicazione, non in un servizio dedicato (diversamente dal nostro approccio)
- **11-12 bit di shard ID sono più comuni di 10 per deployment su larga scala**
- Il trade-off sequence vs shard bits dipende dal numero di nodi e dal carico per nodo
