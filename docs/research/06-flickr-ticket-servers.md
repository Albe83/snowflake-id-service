# Flickr: Ticket Servers — Alternative Approach

**Fonte:** https://code.flickr.net/2010/02/08/ticket-servers-distributed-unique-primary-keys-on-the-cheap/
**Autore:** Kay Kremerskothen, Flickr Engineering
**Data:** 8 Febbraio 2010
**Tipo:** Engineering blog post

## Contesto

Flickr usa MySQL sharding (master-master pairs) e ha bisogno di chiavi primarie globalmente uniche. MySQL auto-increment non funziona tra shard separati.

## Approccio: Ticket Server basato su MySQL

Invece di un algoritmo di generazione, Flickr usa **database server dedicati** che fungono da "distributori di biglietti" (ticket servers).

### Schema

```sql
CREATE TABLE `Tickets64` (
  `id` bigint(20) unsigned NOT NULL auto_increment,
  `stub` char(1) NOT NULL default '',
  PRIMARY KEY  (`id`),
  UNIQUE KEY `stub` (`stub`)
) ENGINE=InnoDB
```

### Operazione

```sql
REPLACE INTO Tickets64 (stub) VALUES ('a');
SELECT LAST_INSERT_ID();
```

`REPLACE INTO` atomically aggiorna una singola riga e restituisce un nuovo ID auto-incrementato.

### Alta Disponibilità

Due ticket server con auto-increment configurati per generare ID alternati:
- **Server 1**: `auto-increment-increment = 2`, `auto-increment-offset = 1` (dispari)
- **Server 2**: `auto-increment-increment = 2`, `auto-increment-offset = 2` (pari)

Round-robin tra i due per load balancing e fault tolerance.

## Confronto con Snowflake

| Aspetto | Flickr Ticket Server | Snowflake |
|---------|---------------------|-----------|
| Coordinamento | Nessuno (ID space diviso staticamente) | Nessuno (machine ID + sequence) |
| Ordinamento temporale | Approssimativo (ordine di generazione) | Alto (timestamp come componente principale) |
| Dipendenze | MySQL | Nessuna (in-memory) |
| Performance | Limitate da MySQL (~60 foto/sec all'epoca) | Decine di migliaia/sec |
| Semplicità | Estremamente semplice | Algoritmo più complesso |

## Rilevanza per il Progetto

- **Alternativa scartata da Twitter** nella progettazione di Snowflake
- Dimostra che per volumi bassi e senza requisiti di ordinamento temporale, un ticket server è più semplice
- Conferma che **l'assenza di coordinamento** è possibile anche con approcci radicalmente diversi
- La tecnica di splitting ID space (pari/dispari) per HA è un pattern riutilizzabile
