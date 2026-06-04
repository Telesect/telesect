# Telesect

Telesect is an integrated, low-overhead communication backbone and operational engine designed for secure, multiplexed data transmission and native telemetry. It operates as a reusable networking primitive, providing a foundational chassis for custom cloud-native transport layouts.

> ⚠️ **Project Scope:** Telesect is an engineering primitive and transport-layer engine. It is **NOT** a chat application or a high-level user messaging platform.

---

## 🛠️ Architectural Pillars & Strict Rules

1. **Pure Go Standard Library:** Zero external dependencies for core networking operations (`net`, `sync`, `crypto/tls`).
2. **Deterministic Resource Reclamation:** Explicit design boundaries for backpressure, strict memory allocation controls, and graceful descriptor teardown.
3. **Verified Performance:** No premature optimization claims without explicit benchmarks (`go test -bench`).
4. **Strict Semantic Versioning:** Moving deliberately from baseline to production matrices (`v0.1.0` target layout).

---

## 🗺️ Engineering Roadmap

- [x] **Phase 1: Relay Foundation**
  - [x] Structured Binary Framing (`v0.1.0` format layout)
  - [x] Full-Duplex Connection Engine (Decoupled Write/Read Worker Loops)
  - [x] Thread-Safe Registry Switchboard
  - [x] Graceful Signal Trapping and Controlled Cascading Teardown
- [ ] **Phase 2: Sub-channel Multiplexing**
- [ ] **Phase 3: Native Telemetry Pipeline**
- [ ] **Phase 4: TLS / Cryptographic Handshake Layer**

---

## 🚀 Getting Started

### Prerequisites
* Go 1.22+ (utilizing standard library execution patterns)

### Building and Running the Backbone

1. Clone the repository into your development directory:
   ```bash
   git clone https://github.com/Telesect/telesect.git
   cd telesect
    ```
2. Execute the test runner with the race detector enabled to verify infrastructure integrity:
   ```bash
   go test -v -race ./...
   ```
3. Spin up the core relay daemon:
   ```bash
   go run cmd/telesectd/main.go
   ```

## Protocol Framing Format
Telesect utilizes a rigid, low-overhead binary layout consisting of a 7-byte fixed header followed by a variable length payload:

```bash
+-------------------+--------------------+--------------------+-----------------------+
|  Stream ID (4B)   |     Type (1B)      |    Length (2B)     |   Payload (Variable)  |
+-------------------+--------------------+--------------------+-----------------------+
```
- Stream ID (4 Bytes): Big-endian unsigned integer representing the virtual sub-channel.

- Type (1 Byte): Protocol control or payload identifier.

- Length (2 Bytes): Explicit sizing metric for the trailing payload (Max 65,535 bytes).

## License
Telesect is open-source software licensed under the Apache License, Version 2.0. See the LICENSE file for full details.