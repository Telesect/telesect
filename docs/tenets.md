# Engineering Tenets

The design, implementation, and evolution of Telesect are governed by a strict set of architectural constraints. These tenets ensure that the backbone remains ultra-lightweight, predictable, and highly secure under heavy network saturation.

---

## 1. Zero Core Dependencies
Telesect is built purely on top of the **Go Standard Library**. 

* **The Rule:** The core networking, synchronization, and security mechanics must only use native packages (such as `net`, `sync`, and `crypto/tls`).
* **The Rationale:** External dependencies introduce supply-chain vulnerabilities, bloat the compiled binary, and complicate long-term maintenance. Telesect aims to be a foundational primitive, meaning it must be as bare-metal and self-contained as possible.

## 2. Strict Semantic Versioning
We respect the dependency graph of our consumers. Telesect strictly adheres to [Semantic Versioning 2.0.0 (SemVer)](https://semver.org/).

* **Initial Phase:** The project starts at version `v0.1.0`.
* **Breaking Changes:** Any modification to the Type-Length-Value (TLV) wire protocol or public connection APIs during minor or patch cycles is strictly forbidden. 

## 3. Data-Driven Optimization (No Guesses)
We do not make hand-wavy claims about performance or throughput. Premature optimization introduces complex code paths that hide bugs.

* **The Rule:** No performance optimization claims will be merged into the main branch without accompanying benchmark data.
* **The Workflow:** Developers must run and document execution traits using Go's testing framework:
  ```bash
  go test -bench=. -benchmem ./...
  ```
## 4. Aggressive Allocation & Memory Limits
To survive heavy throughput without triggering destructive Go garbage collection (GC) pauses, Telesect targets near-zero runtime heap allocations in its hot I/O paths.

- Scratchpad Isolation: Every Connection state instance must utilize pre-allocated, local byte arrays for parsing packet headers.

- Buffer Reusability: Slices used for reading and writing large payloads must be checked back into local sync pools rather than dropped to the heap.

## 5. Defensive Resource Reclamation
Network sockets are finite operating system file descriptors. Telesect treats leaked resources as fatal engine flaws.

- Graceful Degradation: When a connection encounters an I/O error, a malicious packet size, or a flow-control timeout, it must explicitly catch the error, drain pending data safely, reclaim its internal buffers, and atomically close the descriptor to avoid lingering CLOSE_WAIT states.