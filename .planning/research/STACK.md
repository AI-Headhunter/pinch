# Stack Research

**Domain:** Secure agent-to-agent encrypted messaging (Go relay + TypeScript client SDK)
**Researched:** 2026-02-26
**Confidence:** HIGH

## Recommended Stack

### Go Relay Server — Core Technologies

| Technology | Version | Purpose | Why Recommended | Confidence |
|------------|---------|---------|-----------------|------------|
| `github.com/coder/websocket` | v1.8.14 | WebSocket server | Idiomatic Go with native `context.Context` support throughout. Lighter than gorilla/websocket, properly maintained by Coder since 2024 (successor to nhooyr.io/websocket). Context-aware Accept/Dial enables clean cancellation and timeout handling that maps directly to connection lifecycle management. | HIGH |
| `golang.org/x/crypto/nacl/box` | v0.48.0+ | NaCl public-key authenticated encryption | Official Go x/crypto implementation of NaCl crypto_box (XSalsa20-Poly1305 + Curve25519). The relay itself never decrypts, but needs this for protocol-level operations like validating connection handshakes. Imported by 3000+ packages. Interoperable with libsodium. | HIGH |
| `golang.org/x/crypto/nacl/secretbox` | v0.48.0+ | NaCl secret-key authenticated encryption | XSalsa20 + Poly1305 symmetric encryption. Standard NaCl secretbox, interoperable with libsodium-wrappers on the TypeScript side. Used for message encryption after key exchange. | HIGH |
| `crypto/ed25519` | stdlib (Go 1.22+) | Identity signing/verification | Standard library Ed25519 — no external dependency needed. The relay verifies message signatures to authenticate senders without seeing plaintext. RFC 8032 compliant. | HIGH |
| `crypto/ecdh` | stdlib (Go 1.22+) | X25519 key exchange (ECDH) | Standard library X25519 ECDH. Use `ecdh.X25519()` for Diffie-Hellman key agreement. No need for deprecated `golang.org/x/crypto/curve25519` wrapper. | HIGH |
| `filippo.io/edwards25519` | v1.2.0 | Ed25519 to X25519 key conversion | Provides `BytesMontgomery()` to convert Ed25519 public keys to X25519 Montgomery form. Required because Go's standard library does not include Ed25519-to-X25519 conversion. Maintained by Filippo Valsorda (Go crypto team). | HIGH |
| `go.etcd.io/bbolt` | v1.4.3 | Embedded store-and-forward queue | Embedded B+tree key-value store with ACID transactions and single-file storage. Used by NATS Streaming for exactly this pattern (message queue + metadata storage). Simpler than SQLite for key-value queue workloads, no CGO required. Actively maintained by etcd team. | MEDIUM |
| `google.golang.org/protobuf` | v1.36.11 | Message serialization | Protocol Buffers for type-safe, versioned message schemas shared between Go relay and TypeScript client. Schema-first approach prevents Go/TypeScript deserialization drift. Backward/forward compatible field evolution. | HIGH |
| `log/slog` | stdlib (Go 1.21+) | Structured logging | Standard library structured logging. JSON output in production, text locally. Zero dependencies. 650ns/op — adequate for relay server workload. No reason to add zap/zerolog unless profiling shows logging as bottleneck. | HIGH |
| `go-chi/chi` | v5.2.3 | HTTP router (health checks, admin API) | Lightweight, stdlib-compatible router for non-WebSocket HTTP endpoints (health, metrics, admin). Under 1000 LOC core. The WebSocket upgrade itself goes through coder/websocket's `Accept()`, chi just routes the initial HTTP request. | MEDIUM |

### TypeScript Client SDK — Core Technologies

| Technology | Version | Purpose | Why Recommended | Confidence |
|------------|---------|---------|-----------------|------------|
| `libsodium-wrappers-sumo` | 0.8.0 | NaCl cryptography (full API) | The "sumo" variant is required because the standard `libsodium-wrappers` does NOT include `crypto_sign_ed25519_pk_to_curve25519` and `crypto_sign_ed25519_sk_to_curve25519` — the Ed25519-to-X25519 conversion functions that Pinch needs. Compiled to WASM, runs in Node.js and browsers. Official libsodium.js project. | HIGH |
| `@bufbuild/protobuf` | 2.11.0 | Protobuf runtime (message de/serialization) | Runtime for protobuf-es generated code. The only JavaScript Protobuf library that passes full conformance tests. Generates idiomatic TypeScript with plain objects (works with Redux, React Server Components). Paired with `@bufbuild/protoc-gen-es` for code generation. | HIGH |
| `ws` | 8.x | WebSocket client (Node.js) | De facto Node.js WebSocket implementation. Used as the underlying transport for the OpenClaw skill's persistent connection to the relay. Native C++ bindings for performance. Browser environments use native `WebSocket` API instead. | HIGH |

### Shared Protocol Layer

| Technology | Version | Purpose | Why Recommended | Confidence |
|------------|---------|---------|-----------------|------------|
| Protocol Buffers (`.proto` files) | proto3 | Wire format schema definition | Single source of truth for all message types shared between Go and TypeScript. Generates type-safe code for both languages from the same `.proto` files. Schema evolution via field numbering ensures protocol upgrades without breaking compatibility. Lives in `/proto` directory at monorepo root. | HIGH |
| `buf` CLI | latest | Protobuf toolchain | Replaces raw `protoc` with a modern build tool. Handles linting, breaking change detection, and code generation for both Go and TypeScript targets from a single `buf.gen.yaml`. Prevents schema drift between languages. | HIGH |
| `@bufbuild/protoc-gen-es` | 2.11.0 | TypeScript code generator | Generates TypeScript from `.proto` files. Paired with `@bufbuild/protobuf` runtime. Target option `target=ts` for TypeScript output. | HIGH |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `buf` | Proto linting, breaking change detection, code generation | Single `buf.gen.yaml` generates Go + TypeScript from same `.proto` files |
| `go test` + `net/http/httptest` | Relay integration testing | Use `httptest.NewServer` with real WebSocket connections — no mocking library needed. coder/websocket's `Dial` connects to test server for full E2E message flow tests |
| `vitest` or `jest` | TypeScript SDK unit tests | Standard TypeScript test runner for crypto operations and message handling |
| `golangci-lint` | Go linting | Standard Go linting aggregate |
| `buf lint` | Proto schema validation | Catches backwards-incompatible changes before they ship |

## Installation

### Go Relay (`/relay`)

```bash
# Initialize module
go mod init github.com/<org>/pinch/relay

# Core dependencies
go get github.com/coder/websocket@v1.8.14
go get golang.org/x/crypto@latest
go get filippo.io/edwards25519@v1.2.0
go get go.etcd.io/bbolt@v1.4.3
go get google.golang.org/protobuf@v1.36.11
go get github.com/go-chi/chi/v5@v5.2.3
```

### TypeScript SDK (`/skill`)

```bash
npm install libsodium-wrappers-sumo@0.8.0
npm install @bufbuild/protobuf@2.11.0
npm install ws@8

# Dev dependencies
npm install -D @bufbuild/protoc-gen-es@2.11.0
npm install -D @types/ws
npm install -D typescript
```

### Shared Proto Toolchain

```bash
# Install buf CLI
# macOS
brew install bufbuild/buf/buf

# Generate code for both languages
buf generate
```

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| `coder/websocket` | `gorilla/websocket` (v1.5.3) | Gorilla works and is battle-tested, but its API predates Go contexts. coder/websocket has context support throughout, which matters for clean connection lifecycle management (timeouts, cancellation, graceful shutdown). Gorilla is the safe choice if team is already familiar with it. |
| `coder/websocket` | `gobwas/ws` | Better raw performance but significantly more complex API. Only justified at 100K+ concurrent connections where memory per connection matters. Pinch v1 won't hit this. |
| `bbolt` | SQLite via `modernc.org/sqlite` (v1.36.0) | SQLite is more flexible (SQL queries, complex filtering) but adds complexity for what is fundamentally a FIFO queue of encrypted blobs. bbolt's bucket-per-recipient model maps directly to store-and-forward. Consider SQLite if query patterns grow beyond simple enqueue/dequeue. |
| `bbolt` | BadgerDB (v4.x) | Faster writes (~375x vs bbolt in benchmarks) but more complex operation (LSM-tree, compaction, multiple files). bbolt's single-file simplicity is better for a self-hostable relay where operational simplicity matters. |
| Protocol Buffers | MessagePack | Simpler (no schema, "binary JSON") but no cross-language type generation. Schema-first protobuf prevents the Go relay and TypeScript SDK from drifting on message format. The schema IS the protocol spec. |
| Protocol Buffers | FlatBuffers | Zero-copy access is overkill for messages that need to be encrypted/decrypted anyway. Protobuf's ecosystem (buf, protoc-gen-es, protobuf-go) is far more mature. |
| `libsodium-wrappers-sumo` | `tweetnacl` | 31x slower (1,108 ops/s vs libsodium's throughput). Missing Ed25519-to-X25519 conversion functions. Missing `crypto_secretbox`. Not interoperable with Go's `golang.org/x/crypto/nacl`. |
| `libsodium-wrappers-sumo` | `@devtomio/sodium` | Faster in benchmarks (35,092 ops/s) but smaller community and less battle-tested. libsodium-wrappers is the official libsodium.js project maintained by jedisct1 (libsodium author). For a security-critical messaging protocol, provenance matters more than speed. |
| `libsodium-wrappers-sumo` | `sodium-native` | Faster (native C bindings) but requires native compilation, which breaks cross-platform portability. The OpenClaw skill may run on various platforms — WASM-based libsodium-wrappers works everywhere without a C toolchain. |
| `buf` CLI | raw `protoc` | buf handles dependency management, linting, breaking change detection, and multi-target code generation in a single tool. Raw protoc requires manual plugin management and scripting. |
| `log/slog` | `uber-go/zap` | Zap is ~35% faster (420ns vs 650ns) but adds a dependency for marginal gain. slog is stdlib, well-understood, and the Go community standard as of Go 1.21+. Only switch if profiling shows logging latency matters. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `nhooyr.io/websocket` | Deprecated. Replaced by `github.com/coder/websocket`. Same code, new import path. | `github.com/coder/websocket` |
| `golang.org/x/crypto/curve25519` | Frozen wrapper around `crypto/ecdh`. The Go team explicitly migrated X25519 into the standard library. | `crypto/ecdh` with `ecdh.X25519()` |
| `golang.org/x/crypto/ed25519` | Frozen wrapper around `crypto/ed25519` in stdlib. | `crypto/ed25519` (standard library) |
| `tweetnacl` (TypeScript) | 31x slower than libsodium, missing critical conversion functions, smaller primitive set | `libsodium-wrappers-sumo` |
| `libsodium-wrappers` (non-sumo) | Missing `crypto_sign_ed25519_pk_to_curve25519` and `crypto_sign_ed25519_sk_to_curve25519`. Pinch requires these for Ed25519 identity keys to derive X25519 encryption keys. | `libsodium-wrappers-sumo` |
| `github.com/golang/protobuf` | Legacy v1 API. Superseded by `google.golang.org/protobuf` (v2 API). The old module is a thin wrapper that delegates to the new one. | `google.golang.org/protobuf` |
| Custom JSON wire format | No schema enforcement, no type safety across Go/TypeScript, no versioning story. Will cause protocol drift bugs. | Protocol Buffers with `buf` |
| `protobufjs` / `protobuf.js` (TypeScript) | Does not pass Protobuf conformance tests. Less idiomatic TypeScript output. | `@bufbuild/protobuf` (protobuf-es) |
| XMTP protocol | Blockchain-based, decentralized messaging with MLS. Massive overengineering for Pinch's use case. Adds wallet dependencies, mainnet fees, and decentralization complexity that Pinch explicitly scopes out. | Custom lightweight protocol with NaCl primitives |
| External message brokers (Redis, RabbitMQ, NATS) | Pinch relay is self-hostable and single-instance for v1. External brokers add operational complexity and deployment dependencies that contradict "lightweight, self-hostable" constraint. | Embedded bbolt for store-and-forward |

## Stack Patterns by Variant

**If the relay needs to scale beyond single-instance:**
- Add Redis pub/sub for cross-instance message routing
- Replace bbolt with PostgreSQL for durable storage
- This is explicitly OUT OF SCOPE for v1 (see PROJECT.md: "single relay instance for v1")

**If OpenClaw skill needs browser runtime (not just Node.js):**
- `libsodium-wrappers-sumo` already works in browsers (WASM)
- Replace `ws` with native `WebSocket` API (or use `isomorphic-ws` as shim)
- Protocol Buffers via protobuf-es already works in browsers

**If message sizes exceed ~16KB regularly:**
- NaCl secretbox documentation recommends small messages
- Implement chunked encryption at the application layer
- Consider streaming encryption with XChaCha20-Poly1305 (available in libsodium-sumo)

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `coder/websocket` v1.8.14 | Go 1.21+ | Uses generics and context patterns from Go 1.21 |
| `golang.org/x/crypto` v0.48.0 | Go 1.22+ | nacl/box and nacl/secretbox stable, actively maintained |
| `filippo.io/edwards25519` v1.2.0 | Go 1.20+ | Used internally by `crypto/ed25519` |
| `go.etcd.io/bbolt` v1.4.3 | Go 1.22+ | Requires Go 1.22 for v1.4.x series |
| `google.golang.org/protobuf` v1.36.11 | Go 1.21+ | Standard protobuf-go module |
| `libsodium-wrappers-sumo` 0.8.0 | Node.js 14+ | WASM-based, no native dependencies |
| `@bufbuild/protobuf` 2.11.0 | TypeScript 4.9.5+ | ESM and CJS both supported |
| `@bufbuild/protoc-gen-es` 2.11.0 | `@bufbuild/protobuf` 2.11.0 | Must match major.minor version with runtime |

## Crypto Interoperability Matrix

This is the critical compatibility requirement: Go and TypeScript must produce identical ciphertext/signatures for the same inputs.

| Operation | Go Package | TypeScript Package | NaCl Function | Interoperable |
|-----------|-----------|-------------------|---------------|--------------|
| Signing | `crypto/ed25519` | `libsodium-wrappers-sumo` (`crypto_sign`) | `crypto_sign_ed25519` | YES — both RFC 8032 |
| Key exchange | `crypto/ecdh` (X25519) | `libsodium-wrappers-sumo` (`crypto_scalarmult`) | `crypto_scalarmult_curve25519` | YES — both RFC 7748 |
| Ed25519 to X25519 pub | `filippo.io/edwards25519` (`BytesMontgomery`) | `libsodium-wrappers-sumo` (`crypto_sign_ed25519_pk_to_curve25519`) | `crypto_sign_ed25519_pk_to_curve25519` | YES — same math |
| Ed25519 to X25519 priv | SHA-512 first 32 bytes of seed (manual) | `libsodium-wrappers-sumo` (`crypto_sign_ed25519_sk_to_curve25519`) | `crypto_sign_ed25519_sk_to_curve25519` | YES — standard derivation |
| Symmetric encryption | `golang.org/x/crypto/nacl/secretbox` | `libsodium-wrappers-sumo` (`crypto_secretbox`) | `crypto_secretbox_xsalsa20poly1305` | YES — NaCl interop |
| Authenticated encryption | `golang.org/x/crypto/nacl/box` | `libsodium-wrappers-sumo` (`crypto_box`) | `crypto_box_curve25519xsalsa20poly1305` | YES — NaCl interop |

**Critical validation step:** Write cross-language interop tests FIRST (Go encrypts, TypeScript decrypts, and vice versa) before building any protocol logic.

## Sources

- [coder/websocket GitHub](https://github.com/coder/websocket) — verified v1.8.14 as latest release (HIGH confidence)
- [coder/websocket releases](https://github.com/coder/websocket/releases) — release dates and changelog (HIGH confidence)
- [golang.org/x/crypto/nacl/box](https://pkg.go.dev/golang.org/x/crypto/nacl/box) — v0.48.0 verified, function signatures confirmed (HIGH confidence)
- [golang.org/x/crypto/nacl/secretbox](https://pkg.go.dev/golang.org/x/crypto/nacl/secretbox) — v0.48.0 verified, XSalsa20+Poly1305 confirmed (HIGH confidence)
- [crypto/ed25519](https://pkg.go.dev/crypto/ed25519) — standard library, RFC 8032 (HIGH confidence)
- [crypto/ecdh](https://pkg.go.dev/crypto/ecdh) — standard library X25519, RFC 7748 (HIGH confidence)
- [filippo.io/edwards25519](https://pkg.go.dev/filippo.io/edwards25519) — v1.2.0 verified, BytesMontgomery confirmed (HIGH confidence)
- [etcd-io/bbolt releases](https://github.com/etcd-io/bbolt/releases) — v1.4.3 verified (HIGH confidence)
- [protobuf-go releases](https://github.com/protocolbuffers/protobuf-go/releases) — v1.36.11 verified (HIGH confidence)
- [libsodium-wrappers-sumo npm](https://www.npmjs.com/package/libsodium-wrappers-sumo) — v0.8.0 verified (HIGH confidence)
- [@bufbuild/protobuf npm](https://www.npmjs.com/package/@bufbuild/protobuf) — v2.11.0 verified (HIGH confidence)
- [protobuf-es GitHub](https://github.com/bufbuild/protobuf-es) — conformance test status confirmed (HIGH confidence)
- [libsodium Ed25519 to Curve25519 docs](https://doc.libsodium.org/advanced/ed25519-curve25519) — conversion function API (HIGH confidence)
- [Go WebSocket library comparison 2025](https://amf-co.com/which-golang-websocket-library-should-you-use-in-2025/) — ecosystem survey (MEDIUM confidence)
- [go-chi/chi GitHub](https://github.com/go-chi/chi) — v5.2.3 verified (HIGH confidence)
- [Go slog blog post](https://go.dev/blog/slog) — official Go blog (HIGH confidence)
- [buf.build protobuf-es v2 announcement](https://buf.build/blog/protobuf-es-v2) — feature set and conformance (HIGH confidence)
- [goqite — SQLite queue library](https://github.com/maragudk/goqite) — SQLite queue pattern reference (MEDIUM confidence)
- [XMTP protocol overview](https://docs.xmtp.org/protocol/overview) — evaluated and rejected for Pinch's scope (MEDIUM confidence)

---
*Stack research for: Secure agent-to-agent encrypted messaging (Go relay + TypeScript client SDK)*
*Researched: 2026-02-26*
