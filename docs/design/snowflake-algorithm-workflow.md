# Snowflake ID — Algorithm Workflow (128 bit)

> **Decisione:** [MADR 0003](../madr/0003-128-bit-random-payload.md) — 128 bit con payload casuale, stile UUIDv7.
> I miglioramenti #1-#4 (monotonic anchor, hybrid sequence, node ID derivation, lock-free) sono assorbiti dal design a 128 bit e non più necessari.

## Bit Layout

```mermaid
packet-beta
0-47: "Unix timestamp (ms)"
48-51: "Version (0x7)"
52-63: "Random A (12b)"
64-65: "Variant (10)"
66-127: "Random B (62b)"
```
_48 bit timestamp Unix ms + 4 bit version + 2 bit variant + 74 bit random = 128 bit. Conforme a RFC 9562 UUIDv7._

## Startup & Generation Flow

```mermaid
flowchart TD
    subgraph INIT["1. Startup"]
        S["monotonicMs = now()"]
    end

    S --> E

    subgraph GEN["2. ID Generation (NextID)"]
        E["🔒 sync monotonicMs"]
        E --> F["ts = now()"]
        F --> G{"ts > monotonicMs?"}
        G -->|Yes| H["monotonicMs = ts"]
        G -->|No| I["keep monotonicMs"]
        H --> J["🔓 release"]
        I --> J
        J --> K["🔓 fuori dal lock:<br>cryptoRand.Read(10 bytes)"]
        K --> L["set version + variant bits"]
        L --> M["✅ Return 16 bytes"]
    end

    style M fill:#15803d,color:#fff
```

## Sequence Diagram

```mermaid
sequenceDiagram
    box Client
        participant C as Client
    end
    box "Service Scope"
        participant S as Service
        participant G as Generator
    end
    box System
        participant Ck as System Clock
        participant R as crypto/rand
    end

    C->>S: POST /v1/ids {"count": 3}
    S->>G: NextID() × 3

    loop For each ID
        G->>Ck: now()
        Ck-->>G: ts
        alt ts > monotonicMs
            G->>G: monotonicMs = ts
        else ts arretrato
            G->>G: monotonicMs invariato
        end
        G->>R: Read(10 bytes)
        R-->>G: random data
        G->>G: set version=0x7, variant=10
        G-->>S: 16 bytes
    end

    S-->>C: 200 {"ids": ["018f...", "018f...", "018f..."]}
```

## Concurrency Model

```mermaid
flowchart LR
    subgraph "Goroutine 1"
        G1["NextID()"]
    end
    subgraph "Goroutine 2"
        G2["NextID()"]
    end
    subgraph "Goroutine N"
        GN["NextID()"]
    end

    G1 --> M["🔒 sync.Mutex<br>solo su monotonicMs"]
    G2 --> M
    GN --> M
    M --> Gen["monotonicMs"]
    Gen --> R1["crypto/rand (no lock)"]
    Gen --> R2["crypto/rand (no lock)"]
    Gen --> RN["crypto/rand (no lock)"]
    R1 --> ID1["✅ 16 bytes"]
    R2 --> ID2["✅ 16 bytes"]
    RN --> IDN["✅ 16 bytes"]
```
_Il mutex protegge solo l'avanzamento di `monotonicMs`. La generazione random (la parte costosa) avviene fuori dal lock, in parallelo._

## Pseudocodice

```go
type Generator struct {
    mu          sync.Mutex
    monotonicMs int64
}

func NewGenerator() *Generator {
    return &Generator{monotonicMs: nowMs()}
}

func (g *Generator) NextID() ([]byte, error) {
    ts := nowMs()
    g.mu.Lock()
    if ts > g.monotonicMs {
        g.monotonicMs = ts
    }
    ts = g.monotonicMs
    g.mu.Unlock()

    var id [16]byte
    binary.BigEndian.PutUint48(id[0:6], uint64(ts))
    cryptoRand.Read(id[6:16])
    id[6] = (id[6] & 0x0F) | 0x70  // version 7
    id[8] = (id[8] & 0x3F) | 0x80  // variant 10xx
    return id[:], nil
}
```

## Cosa NON serve più

| Meccanismo (necessario a 64 bit) | Perché non serve a 128 bit |
|---|---|
| **Node ID** | 74 bit random garantiscono unicità senza identificare la macchina |
| **Sequence + overflow** | Infiniti ID per ms (2^74 spazio casuale) |
| **Clock skew policy** | Anche col clock che torna indietro, random rende il duplicato impossibile |
| **Persistenza cross-restart** | Random diverso a ogni restart, collisione ~impossibile |
| **Spin-wait** | Non esiste più il concetto di "sequence esaurita" |
| **Configurazione manuale** | Zero env, zero file, zero Zookeeper |
| **Epoch custom** | Unix epoch standard, ID auto-descrittivo |

## Serializzazione

Gli ID a 128 bit sono binari (16 byte). Per la risposta JSON, due opzioni:

| Formato | Esempio | Dimensione |
|---|---|---|
| **Hex canonico** (UUID standard) | `018f3a2c-9e5b-7000-8000-123456789abc` | 36 caratteri |
| **Base62** (KSUID-style) | `0ujsswThIGTUYm2K8FjOOfXtY1K` | 22 caratteri |
| **Base64 URL-safe** | `AY86LJ5bcACAAAAAABI0VniavA` | 22 caratteri |

Raccomandato: **hex canonico** per interoperabilità con UUIDv7. Se la compattezza è critica, Base62.
