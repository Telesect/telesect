# Telesect

Telesect is an integrated, low-overhead communication backbone and operational engine designed for secure, multiplexed data transmission and native telemetry. It operates as a reusable networking primitive, providing a foundational chassis for custom cloud-native transport layouts.

> ⚠️ **Project Scope:** Telesect is an engineering primitive and transport-layer engine. It is **NOT** a chat application or a high-level user messaging platform.

---

## 🛠️ Architectural Pillars & Strict Rules

1. **Pure Go Standard Library:** Zero external dependencies for core networking operations (`net`, `sync`, `crypto/tls`).
2. **Deterministic Resource Reclamation:** Explicit design boundaries for backpressure, strict memory allocation controls, and graceful descriptor teardown.
3. **Verified Performance:** No premature optimization claims without explicit benchmarks (`go test -bench`).
4. **Strict Semantic Versioning:** Moving deliberately from baseline to production matrices (`v0.1.0` starting layout).

---

## 🗺️ Engineering Roadmap

### 📦 Phase 1: Relay Foundation — **[COMPLETE]**
- [x] Full-Duplex Connection Engine (Decoupled Write/Read Worker Loops)
- [x] Thread-Safe Registry Switchboard (`sync.RWMutex` protected)
- [x] Graceful Signal Trapping (SIGINT/SIGTERM) and Controlled Cascading Teardown

### 🎛️ Phase 2: The Protocol & Framing Layer — **[IN PROGRESS]**
- [x] **Milestone 2.1 — TLV Binary Framing & Hardening**
  - [x] Implement rigid 5-byte wire contract with Network Byte Order (Big-Endian) serialization.
  - [x] Build defensive OOM mitigation and maximum frame ceiling validation checks.
  - [x] Route validated packet ingress streams to the central master `InboundRouter` switchboard channel.
- [ ] **Milestone 2.2 — Application-Layer Flow Control & Backpressure**
  - [ ] Introduce explicit Control Frames (`0x05 WINDOW_UPDATE`).
  - [ ] Implement queue saturation tracking and upstream execution throttling.
- [x] **Milestone 2.3 — Measurement & Profiling Baseline**
  - [x] Establish high-throughput micro-benchmarks (`go test -bench -benchmem`).
  - [ ] Profile hot paths using `pprof` and optimize allocations using pooling strategies *after* Milestone 2.2.

### 📊 Phase 3: Native Telemetry Pipeline — **[BACKLOG]**
### 🔒 Phase 4: TLS / Cryptographic Handshake Layer — **[BACKLOG]**

---

## 🚀 Getting Started

### Prerequisites
* Go 1.22+ (utilizing standard library execution patterns)

### Building and Running the Backbone

1. Clone the repository into your development directory:
   ```bash
   git clone [https://github.com/Telesect/telesect.git](https://github.com/Telesect/telesect.git)
   cd telesect
   ```
2. Execute the test runner with the runtime race detector enabled to verify infrastructure integrity:
   ```bash
   go test -v -race ./...
   ```
3. Run the micro-throughput benchmarks:
   ```bash
   go test -bench=BenchmarkServer_Throughput -benchmem ./internal/transport
   ```
4. Spin up the core relay daemon
   ```bash
   go run cmd/telesectd/main.go
   ```
## Protocol Framing Format

```bash
+-----------+-----------------------+-----------------------+
| Type (1B) |      Length (4B)      |   Value (Variable)    |
+-----------+-----------------------+-----------------------+
|  Offset 0 |      Offsets 1-4      |       Offset 5+       |
+-----------+-----------------------+-----------------------+
```

- Type (1 Byte / Offset 0): Transparent routing identifier separating internal control signals from application traffic vectors.

   - 0x01 - Control Plane Signal

   - 0x02 - Agritech/Telemetry Data Vector

   - 0x05 - Window Update / Flow Control Frame

- Length (4 Bytes / Offsets 1-4): Big-endian 32-bit unsigned integer defining the explicit sizing boundaries of the trailing payload.

- Value (Variable Sizing / Offset 5+): Raw application or network command bytes.

## 📈 Performance Log & Baselines

To respect Architectural Pillar #3, performance mutations are logged systematically against baseline engineering milestones.

### Baseline Run: Milestone 2.1 Complete (Raw Ingestion Floor)
- Environment: AMD Ryzen 3 3250U (4 Logical Threads), Linux amd64

- Metrics:

   - Latency: 26884 ns/op (~37,000 packets/sec end-to-end loopback processing)

   - Memory Overhead: 138 B/op

   - Heap Allocations: 4 allocs/op
  
## License
Telesect is open-source software licensed under the Apache License, Version 2.0. See the LICENSE file for full details.