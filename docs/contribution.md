# Contribution Guide

Thank you for your interest in improving Telesect! We are building a secure, high-performance, and completely self-contained networking primitive. To maintain the structural integrity of the codebase, all contributors must follow the guidelines detailed below.

---

## Code Review Checklist

Before opening a Pull Request (PR), verify your changes align with our core engineering tenets:

* **Zero External Dependencies:** Your changes must use the Go Standard Library exclusively. PRs adding third-party modules or external packages to the core engine will be automatically closed.
* **Format Compliance:** All source files must be properly linted and formatted via the native toolchain:

  ```bash
  go fmt ./...

  ```

* **Error Handling:** Avoid silencing errors using blank identifiers (`_`). Every network anomaly, frame parsing error, and socket state failure must be caught, handled defensively, and wrapped explicitly.

---

## Performance & Benchmarking Rules

Telesect does not accept performance assertions based on intuition. If your pull request modifies hot code paths (such as `pkg/protocol` frame serialization or `internal/transport` loop dynamics), you must provide empirical execution metrics.

### 1. Run Local Memory Allocation and Speed Benchmarks

Before and after making your modifications, execute the internal benchmarking suite:

```bash
go test -bench=. -benchmem ./...

```

### 2. Document Your Claims

In your pull request description, you must provide a comparative table showing the impact of your code change:

| Metric | Main Branch (Before) | Your Branch (After) | Delta |
| --- | --- | --- | --- |
| **Throughput Speed** | `X ns/op` | `Y ns/op` | Z |
| **Heap Allocations** | `A B/op` | `B B/op` | C|
| **Alloc Objects** | `N allocs/op` | `M allocs/op` |  K |

We strictly optimize for **minimal allocation objects (`allocs/op`)** to prevent the Go garbage collector from triggering unpredictably under intense network throughput.

---

## Pull Request Lifecycle

1. **Fork and Branch:** Create a feature branch originating from the latest `main` branch. Use clear, descriptive names (e.g., `feature/tlv-padding-validation` or `fix/connection-leak-on-timeout`).
2. **Write Unit Tests:** Ensure your logic changes are completely verified by accompanying unit tests in the same directory.
3. **Verify Documentation:** If your changes modify user-facing behaviors or add a control frame type, update the corresponding `docs/protocol_schema.md` or `docs/flow_backpressure.md` files.
4. **Submit for Review:** Once all tests pass and benchmarks are attached, submit your PR. A project maintainer will review your implementation details against our strict engineering tenets.
