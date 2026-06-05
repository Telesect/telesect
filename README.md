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

### 🎛️ Phase 2: The Protocol & Framing Layer — **[COMPLETE]**
- [x] **Milestone 2.1 — TLV Binary Framing & Hardening**
  - [x] Implement rigid 5-byte wire contract with Network Byte Order (Big-Endian) serialization.
  - [x] Build defensive OOM mitigation and maximum frame ceiling validation checks.
  - [x] Route validated packet ingress streams to the central master `InboundRouter` switchboard channel.
- [x] **Milestone 2.2 — Application-Layer Flow Control & Backpressure**
  - [x] Introduce explicit Control Frames (`0x05 WINDOW_UPDATE`).
  - [x] Implement queue saturation tracking and upstream execution throttling.
- [x] **Milestone 2.3 — Measurement & Profiling Baseline**
  - [x] Establish high-throughput micro-benchmarks (`go test -bench -benchmem`).
  - [x] Profile hot paths using `pprof` and optimize allocations using pooling strategies (`sync.Pool` and fixed-size slice arrays).

### 🔒 Phase 3: The Secure Trust Layer — **[UP NEXT]**
- [ ] Milestone 3.1 — mTLS Transport & Identity Binding
- [ ] Milestone 3.2 — Stream-Level Role-Based Access Control (RBAC)
- [ ] Milestone 3.3 — Perimeter Defenses & Traffic Operations

### 📊 Phase 4: Native Telemetry & TUI Control Plane — **[BACKLOG]**

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

- Type (1 Byte / Offset 0): A transparent routing identifier. The engine enforces a strict boundary between internal transport operations and external application payloads.

   - 0x00 - 0x0F (Internal Engine Reserved): Exclusively intercepted and processed by the transport layer. External applications never see these.

      - Active: 0x05 - Window Update / Flow Control Frame.

- 0x10 - 0xFF (Application Space): 100% payload-agnostic. The engine transparently routes these to the central switchboard. Developers define their own contracts here.

   - Example Use Case: 0x10 - Health/Diagnostic Service

   - Example Use Case: 0x11 - Agritech/Telemetry Data Vector

   - Example Use Case: 0x12 - TUI Control Plane Command

- Length (4 Bytes / Offsets 1-4): Big-endian 32-bit unsigned integer defining the explicit sizing boundaries of the trailing payload (Max: 16MB).

- Value (Variable Sizing / Offset 5+): Raw application or network command bytes.

## 📈 Performance Log & Baselines

To respect Architectural Pillar #3, performance mutations are logged systematically against baseline engineering milestones.

### Baseline Run: Milestone 2.1 Complete (Raw Ingestion Floor)
- Environment: AMD Ryzen 3 3250U (4 Logical Threads), Linux amd64

- Metrics:

   - Latency: 26884 ns/op (~37,000 packets/sec end-to-end loopback processing)

   - Memory Overhead: 138 B/op

   - Heap Allocations: 4 allocs/op

### Optimized Run: Milestone 2.3 Complete (Zero-Allocation Hot Path)
- Environment: AMD Ryzen 3 3250U (4 Logical Threads), Linux amd64

- Metrics:

   - Latency: 26,118 ns/op

   - Memory Overhead: 0 B/op

   - Heap Allocations: 0 allocs/op

Notes: Achieved via sync.Pool packet recycling and fixed-size struct slice arrays for header bounds checking.
  
## License
Telesect is open-source software licensed under the Apache License, Version 2.0. See the LICENSE file for full details.