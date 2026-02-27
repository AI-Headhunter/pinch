# Pitfalls Research

**Domain:** Secure agent-to-agent encrypted messaging protocol (Go relay + TypeScript skill + NaCl crypto)
**Researched:** 2026-02-26
**Confidence:** HIGH (well-documented failure modes in cryptography, WebSocket systems, and agent communication)

## Critical Pitfalls

### Pitfall 1: Nonce Reuse in NaCl secretbox/crypto_box

**What goes wrong:**
Reusing a nonce with the same key in NaCl secretbox completely destroys confidentiality. An attacker who observes two ciphertexts encrypted with the same (key, nonce) pair can XOR them to cancel the keystream and recover plaintext. This is not a theoretical attack -- it is trivially exploitable.

**Why it happens:**
Counter-based nonce schemes lose state across restarts. Random nonces collide if the RNG is weak or the nonce space is too small. Developers test with a single sender and miss that both sides of a channel might generate the same nonce value. In store-and-forward systems, messages may be re-encrypted for retransmission with a stale nonce counter.

**How to avoid:**
- Use 24-byte random nonces (NaCl's default). With 192 bits, random collision probability is negligible even at billions of messages.
- Never use counter-based nonces unless the counter is persisted atomically with the key and is never reset.
- If counters are used, assign odd counters to one side and even to the other (or use distinct prefixes) to prevent cross-party collision.
- Prepend the nonce to the ciphertext (standard NaCl pattern) so the receiver always has it.

**Warning signs:**
- Nonce generation code that does not call `randombytes_buf()` or equivalent CSPRNG.
- Any nonce variable initialized to zero without persistent storage.
- Tests passing with hardcoded nonces (tests should use random nonces like production).

**Phase to address:**
Phase 1 (core crypto layer) -- nonce handling must be correct from the first encrypted message.

---

### Pitfall 2: Ed25519-to-X25519 Key Conversion as Single Point of Failure

**What goes wrong:**
Pinch plans to use one Ed25519 keypair for both signing and encryption (converting to X25519 for key exchange). If the signing key is compromised, both authentication AND all encrypted messages are compromised simultaneously. Libsodium's own documentation notes that "using the same key pair for key exchange and signing is not recommended" and that "conversions are no longer necessary." Additionally, a 2025 vulnerability in `crypto_core_ed25519_is_valid_point()` (CVE-2025-69277) showed that low-level validation can have subtle bugs, though high-level APIs were unaffected.

**Why it happens:**
Single-keypair identity is simpler -- one address, one key to manage, one backup to secure. The conversion functions exist in libsodium, which makes it feel "officially supported." The PROJECT.md explicitly plans this approach: "Same keypair for signing (Ed25519) and encryption (convert to X25519)."

**How to avoid:**
- Accept the tradeoff for v1 (PROJECT.md already scopes this), but design the protocol envelope to support key separation later. Include a `signing_key` and `encryption_key` field in the identity/connection handshake, even if they derive from the same source in v1.
- Ensure the conversion uses `crypto_sign_ed25519_pk_to_curve25519()` and `crypto_sign_ed25519_sk_to_curve25519()` -- never hand-roll the conversion.
- Add protocol versioning from day one so a future version can require separate keypairs without breaking existing connections.

**Warning signs:**
- Identity format that only has room for one key (no extensibility for a second).
- Protocol messages that assume signing key IS the encryption key with no indirection layer.

**Phase to address:**
Phase 1 (identity and keypair management). Design the protocol envelope extensibly even if v1 uses a single keypair.

---

### Pitfall 3: Group Encryption Without Key Rotation on Member Removal

**What goes wrong:**
When a member is removed from a group channel, they retain the group symmetric key. Without rotating the key, the removed member can decrypt all future messages if they retain access to the encrypted ciphertext (e.g., through a compromised relay or network sniffing). This is the most common security failure in group E2E implementations.

**Why it happens:**
Key rotation on member removal requires re-encrypting the new key to every remaining member and is operationally expensive. In store-and-forward systems, there is a window between removal and key rotation where queued messages use the old key. Developers defer this to "later" and ship without it, creating a permanent backdoor for every removed member.

**How to avoid:**
- On member removal: generate a new group symmetric key, encrypt it to each remaining member's public key, distribute the new key, and mark a message sequence boundary.
- Messages queued before removal use the old key (acceptable -- member had access then). Messages after the boundary MUST use the new key.
- For v1 with NaCl box: use a "sender keys" approach where each member has their own symmetric key shared with all other members. Removing a member means everyone except the removed member generates a new sender key.

**Warning signs:**
- Group key stored as a single long-lived value with no rotation mechanism.
- Member removal API that only updates a membership list without touching crypto state.
- No message sequence numbers or epoch boundaries in the group protocol.

**Phase to address:**
Group channels phase. This must be designed before groups ship, not retrofitted.

---

### Pitfall 4: WebSocket Connection Leak and Goroutine Exhaustion in Go Relay

**What goes wrong:**
Each WebSocket connection in Go spawns at least one goroutine (often two: one for reading, one for writing). Connections that hang without cleanly closing -- due to network partitions, client crashes, or mobile connectivity loss -- pin goroutines and their associated memory indefinitely. gorilla/websocket has documented issues where 1,000 clients at moderate message rates consumed 10 GB of memory in 2 minutes due to goroutine/buffer leaks.

**Why it happens:**
TCP keepalive defaults are too slow (often 2+ hours) to detect dead connections. Developers implement the "happy path" (clean close) but not the "sad path" (unresponsive client, network drop). The Go garbage collector cannot reclaim goroutines blocked on a read from a dead connection.

**How to avoid:**
- Implement WebSocket ping/pong heartbeats at 20-30 second intervals with a 5-10 second pong timeout. Close connections that miss 2 consecutive pongs.
- Set read/write deadlines on every WebSocket operation: `conn.SetReadDeadline(time.Now().Add(60 * time.Second))`.
- Use `context.Context` for connection lifecycle management (nhooyr.io/websocket does this natively).
- Monitor goroutine count (`runtime.NumGoroutine()`) and expose it as a metric. Alert if it trends upward without corresponding connection growth.
- Set per-connection write buffers and use `conn.SetWriteDeadline()` to prevent slow-consumer backpressure from blocking the write goroutine.

**Warning signs:**
- Goroutine count climbing over time without connection count increasing proportionally.
- Memory usage growing steadily without plateauing.
- No `SetReadDeadline`/`SetWriteDeadline` calls in the WebSocket handler code.
- No ping/pong handler registered.

**Phase to address:**
Phase 1 (relay server foundation). Connection lifecycle management must be correct before any load testing.

---

### Pitfall 5: Replay Attacks on Store-and-Forward Messages

**What goes wrong:**
An attacker (or a compromised relay) captures encrypted message blobs and replays them. Without replay protection, the recipient processes duplicate messages. In an agent context, this can cause duplicate action executions, duplicate confirmations, or state corruption. Even though the relay is "cryptographically blind," it handles opaque blobs and could replay them.

**Why it happens:**
NaCl authenticated encryption (crypto_box / secretbox) guarantees confidentiality and authenticity but NOT uniqueness. A valid ciphertext is valid every time it is decrypted. Developers assume "encrypted = safe" without considering that valid encrypted messages can be replayed. Store-and-forward makes this worse because messages are already designed to be stored and delivered later.

**How to avoid:**
- Include a monotonically increasing sequence number inside the encrypted payload (not in the cleartext envelope where the relay could modify it).
- Recipients track the highest sequence number seen per sender per channel and reject anything at or below it.
- Include a timestamp inside the encrypted payload. Recipients reject messages older than a configurable window (e.g., 7 days for store-and-forward tolerance).
- The relay should also deduplicate by message ID in the cleartext envelope, but this is defense-in-depth only -- the real protection must be inside the encryption boundary.

**Warning signs:**
- Message format that has no sequence number or message ID inside the encrypted payload.
- Recipient code that processes every successfully-decrypted message without dedup checks.
- Tests that don't exercise "same message delivered twice" scenarios.

**Phase to address:**
Phase 1 (message format design). The encrypted payload structure must include replay protection fields from the start.

---

### Pitfall 6: Autonomy Level Escalation Without Audit Trail

**What goes wrong:**
Pinch's 4-tier autonomy model (Full Manual -> Notify -> Auto-respond -> Full Auto) means an agent at "Full Auto" can send messages and take actions without human approval. If autonomy level changes are not logged, if there is no confirmation for escalation, or if the default is too permissive, agents act without oversight and humans have no way to detect or reverse harmful actions.

**Why it happens:**
Developers default to "Full Auto" during development for convenience and forget to change the default. The autonomy level is set once during connection approval and never revisited. There is no mechanism for a human to see what their agent has been doing at higher autonomy levels. Research shows nearly 80% of organizations deploying autonomous AI cannot tell in real time what their agents are doing.

**How to avoid:**
- Default autonomy level for new connections MUST be "Full Manual" (most restrictive). Escalation requires explicit human action.
- Every autonomy level change is logged in the audit trail with timestamp, old level, new level, and who/what triggered it.
- Activity feed (already planned) must surface all agent actions, with escalating visual priority for higher-autonomy actions.
- Implement a "cooldown" or confirmation step when escalating to Auto-respond or Full Auto: "This connection will now send messages without your approval. Confirm?"
- Circuit breakers (already planned) should auto-downgrade autonomy if anomalous patterns are detected (e.g., sudden burst of outbound messages).

**Warning signs:**
- Default autonomy level is anything other than the most restrictive.
- Autonomy changes not appearing in audit logs.
- No human-visible summary of what an agent did during a session at higher autonomy.
- Testing only at one autonomy level.

**Phase to address:**
Connection consent phase AND autonomy phase. The data model for autonomy must include audit fields from day one.

---

### Pitfall 7: Metadata Leakage Despite E2E Encryption

**What goes wrong:**
E2E encryption protects message content, but the relay necessarily sees: who communicates with whom, when, message sizes, frequency patterns, and online/offline status. For an agent-to-agent system, this metadata reveals which agents (and therefore which humans/organizations) are collaborating, project timelines (activity bursts), and relationship graphs. This is the exact metadata that sophisticated adversaries (or a compromised relay operator) would exploit.

**Why it happens:**
The focus on encrypting content creates a false sense of total privacy. The relay needs addressing information to route messages, so some metadata is inherently visible. Developers don't consider metadata as a threat because they equate "encrypted" with "private."

**How to avoid:**
- Acknowledge this in the threat model explicitly. Pinch's relay is self-hosted, which limits the exposure to the relay operator (who is also the user), but a compromised relay is still a threat.
- Minimize metadata: use fixed-size padded messages to hide content length, batch messages where possible, avoid exposing precise timestamps in the protocol (use epoch-level granularity if feasible).
- Connection metadata (who connects to whom) is visible to the relay -- this is an accepted tradeoff for v1. Document it clearly.
- Do NOT log message metadata at the relay beyond what is needed for routing/delivery confirmation.

**Warning signs:**
- Relay logging that includes sender, recipient, timestamp, and message size for every message.
- Variable-length encrypted blobs that reveal content type (text vs. file) by size.
- No threat model document addressing metadata exposure.

**Phase to address:**
Phase 1 (relay design) for minimization; documentation phase for threat model. Full metadata protection (padding, traffic analysis resistance) is post-v1.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Single keypair for signing + encryption | Simpler identity, one address | Compromised key breaks both auth and confidentiality; blocks advanced crypto features | v1 only, with protocol extensibility for future key separation |
| No forward secrecy (static key exchange) | Much simpler implementation; no ratchet state | Compromised long-term key decrypts all past messages | v1 only; PROJECT.md explicitly defers this. Upgrade path must exist |
| In-memory message queue at relay | No database dependency, fast | Messages lost on relay crash/restart | Early development only. Must add persistent store before any production use |
| Shared group symmetric key (vs. sender keys or MLS) | Simple group crypto | O(n) re-encryption on member change, no per-sender authentication within group | v1 with small groups (<20 members). Plan migration to sender keys |
| Single relay instance (no federation) | No discovery, no routing complexity | Single point of failure, no horizontal scaling | v1 explicitly. Fine for self-hosted single-org use case |
| Hardcoded crypto algorithms | No negotiation complexity | Cannot upgrade algorithms without protocol break | Never acceptable without protocol versioning. Add version field from day one |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| libsodium-wrappers (TypeScript) | Calling crypto functions before `await sodium.ready` resolves; causes silent failures or exceptions | Always `await sodium.ready` at module initialization. Wrap in a singleton that blocks until ready |
| OpenClaw heartbeat cycle | Assuming heartbeat fires at exact intervals; building timing-dependent logic | Treat heartbeat as "at least this often" -- design for variable intervals and missed beats |
| WebSocket upgrade behind reverse proxy | Proxy (nginx, CloudFlare) drops WebSocket connections at its own idle timeout (often 60s) | Set heartbeat interval below the proxy's idle timeout. Document required proxy config for self-hosters |
| Go crypto/nacl vs. libsodium | Assuming Go's `golang.org/x/crypto/nacl` and libsodium produce identical output for all operations | Test cross-language encrypt/decrypt in CI. Subtle differences exist in key validation and padding |
| Ed25519 signature verification across languages | Different libraries may handle the "malleability" check differently (cofactor vs. cofactorless verification) | Pin to specific library versions. Add cross-language signature verification tests |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-message public-key encryption for groups | Latency increases linearly with group size; 100-member group = 100 crypto_box operations per message | Use symmetric group key; encrypt the group key to each member only on key changes (member add/remove) | >10 members per group |
| Unbounded store-and-forward queue | Relay memory grows without bound for offline agents; one agent offline for days consumes relay RAM/disk | Set per-agent queue depth limit (e.g., 10,000 messages). Oldest messages expire after configurable TTL (e.g., 7 days). Return "mailbox full" to senders | Any agent offline for extended period |
| Synchronous crypto in WebSocket message handler | Message processing blocks the connection's read loop; backpressure causes write buffer growth | Offload encryption/decryption to a worker goroutine/pool. Keep WebSocket read/write loops non-blocking | >100 concurrent connections |
| JSON serialization of binary crypto data | Base64 encoding inflates message size by 33%; JSON parsing of large messages is CPU-intensive | Use binary framing (protobuf or MessagePack) for the wire format. JSON only for human-readable debug/API surfaces | >1,000 messages/second |
| No connection pooling for relay-to-agent delivery | Creating new goroutine per delivery attempt for queued messages on agent reconnect | Use a bounded worker pool for draining stored messages. Rate-limit delivery to prevent reconnection thundering herd | >50 agents reconnecting simultaneously |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Trusting cleartext envelope fields for routing without rate limiting | Relay DoS: attacker floods messages to a target agent address. No content inspection possible (E2E), so relay blindly queues | Rate limit per source address at the relay. Implement circuit breakers per connection pair |
| Storing private keys in plaintext on disk | Full identity compromise if filesystem is accessed | Encrypt private key at rest with a user-provided passphrase or OS keychain integration. Never log key material |
| Not validating public keys received during connection handshake | Invalid or malicious public key could cause crypto operations to fail silently or produce predictable output | Validate all received public keys with `crypto_core_ed25519_is_valid_point()` (or equivalent). Reject connections with invalid keys |
| Connection consent bypass via relay modification | A malicious relay could forge connection acceptance messages since the relay routes handshake messages | Sign connection requests and acceptances with the sender's Ed25519 key. Verify signatures at the receiving skill, not at the relay |
| Cleartext error messages revealing internal state | Error responses from relay or skill that include key fragments, internal paths, or crypto operation details | Sanitize all error messages. Use generic error codes. Log details server-side only |
| No certificate pinning or relay authentication | MITM on the WebSocket connection (even with TLS) if the agent trusts any certificate | Pin relay TLS certificate or public key in the skill configuration. Verify relay identity on connection |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No feedback on message delivery status | Human has no idea if their agent's message was delivered, queued, or lost | Implement delivery receipts: sent -> relayed -> delivered -> read. Show in activity feed |
| Autonomy level names that don't convey risk | "Full Auto" sounds efficient, not dangerous. Humans escalate without understanding implications | Use descriptive names: "Full Auto" -> "Unsupervised" or "Agent Acts Alone." Include risk description in the escalation prompt |
| Connection request with no context | Agent receives `pinch:abc123@relay` wants to connect -- no information about who or why | Include human-readable metadata in connection requests: display name, purpose, referring context |
| Activity feed that requires polling/checking | Humans forget to check. High-autonomy agent actions go unreviewed | Push notifications for notable events. Digest summaries for quiet periods. Escalation alerts for anomalies |
| Key backup/recovery not addressed | Lost device = lost identity = lost all connections. Recreating identity requires re-establishing every connection | Provide encrypted key export/import. Warn users during setup. Make backup flow part of onboarding |

## "Looks Done But Isn't" Checklist

- [ ] **E2E encryption:** Often missing replay protection inside the encrypted payload -- verify sequence numbers are included AND checked on receipt
- [ ] **Connection handshake:** Often missing mutual authentication -- verify both sides sign and verify the handshake, not just the initiator
- [ ] **Store-and-forward:** Often missing message expiration -- verify queued messages have TTL and are purged after expiry
- [ ] **Group channels:** Often missing key rotation on member removal -- verify removing a member triggers new group key distribution
- [ ] **WebSocket connection:** Often missing dead connection cleanup -- verify ping/pong timeouts are implemented and goroutine counts are monitored
- [ ] **Autonomy levels:** Often missing audit trail -- verify every autonomy-level action and change is logged with timestamp
- [ ] **Rate limiting:** Often missing per-connection granularity -- verify limits are per source-destination pair, not just global
- [ ] **Error handling:** Often missing crypto operation failure paths -- verify what happens when decryption fails, signature verification fails, key conversion fails
- [ ] **Cross-language crypto:** Often missing interop tests -- verify Go relay and TypeScript skill can round-trip encrypt/decrypt/sign/verify
- [ ] **Blocking/muting:** Often missing enforcement at relay level -- verify blocked connections cannot send messages through the relay, not just that the skill ignores them

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Nonce reuse discovered in production | HIGH | Rotate all affected keys immediately. Re-encrypt and re-send affected messages. Audit all messages encrypted with the compromised nonce/key pair for potential exposure |
| Group key not rotated after member removal | MEDIUM | Generate new group key, distribute to remaining members, mark epoch boundary. Past messages remain accessible to removed member (cannot be undone) |
| WebSocket goroutine leak in production | LOW | Deploy fix with proper timeouts. Existing leaked goroutines will be cleaned up on relay restart. Add monitoring to prevent recurrence |
| Replay attack exploited | MEDIUM | Add sequence number checking. Audit logs for duplicate message processing. Idempotency in agent action handlers limits damage |
| Private key exposed | HIGH | Revoke the compromised identity. Generate new keypair. Re-establish all connections. Notify all contacts of identity change. All past encrypted messages to/from this identity should be considered compromised |
| Metadata leak via relay logs | LOW-MEDIUM | Purge logs. Reduce relay logging to minimum necessary. Audit log retention policies. Cannot undo exposure of already-logged metadata |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Nonce reuse | Phase 1: Core crypto layer | Unit tests with nonce uniqueness assertions; cross-language encrypt/decrypt roundtrip tests |
| Single keypair risk | Phase 1: Identity design | Protocol envelope includes separate signing_key and encryption_key fields (even if same value in v1) |
| Group key rotation | Group channels phase | Integration test: remove member, send message, verify removed member cannot decrypt |
| WebSocket connection leak | Phase 1: Relay server | Load test with abrupt disconnections; goroutine count monitoring in CI |
| Replay attacks | Phase 1: Message format | Integration test: deliver same encrypted blob twice, verify second is rejected |
| Autonomy escalation | Connection consent / autonomy phase | Audit log entries for every autonomy change; default is Full Manual in tests |
| Metadata leakage | Phase 1: Relay design + docs | Relay logs reviewed for metadata exposure; threat model document created |
| Store-and-forward queue exhaustion | Phase 1: Relay server | Load test with offline agents; verify queue depth limits and TTL expiration |
| Cross-language crypto mismatch | Phase 1: Core crypto layer | CI pipeline with Go-encrypts/TS-decrypts and TS-encrypts/Go-decrypts test matrix |
| libsodium-wrappers async init | Phase 1: Skill crypto layer | Test that crypto operations called before `sodium.ready` throw clear errors, not silent failures |

## Sources

- [Libsodium: Authenticated encryption (secretbox)](https://libsodium.gitbook.io/doc/secret-key_cryptography/secretbox) -- nonce handling requirements
- [Libsodium: Ed25519 to Curve25519 conversion](https://libsodium.gitbook.io/doc/advanced/ed25519-curve25519) -- key conversion caveats
- [Ed25519 key validation vulnerability (CVE-2025-69277)](https://www.miggo.io/vulnerability-database/cve/CVE-2025-69277) -- 2025 libsodium vulnerability
- [Frank Denis: A vulnerability in libsodium (Dec 2025)](https://00f.net/2025/12/30/libsodium-vulnerability/) -- ed25519 point validation bypass
- [Thormarker (2021): On using the same key pair for Ed25519 and X25519](https://eprint.iacr.org/2021/509.pdf) -- security analysis of key reuse
- [gorilla/websocket memory issues (#134, #236, #273, #296)](https://github.com/gorilla/websocket/issues/134) -- Go WebSocket memory pitfalls
- [Challenges of scaling WebSockets (Ably)](https://dev.to/ably/challenges-of-scaling-websockets-3493) -- distributed WebSocket patterns
- [WebSocket Scale in 2025 (VideoSDK)](https://www.videosdk.live/developer-hub/websocket/websocket-scale) -- scaling architecture
- [Trail of Bits: Better Encrypted Group Chat](https://blog.trailofbits.com/2019/08/06/better-encrypted-group-chat/) -- group encryption pitfalls
- [p2panda: Group encryption (2025)](https://p2panda.org/2025/02/24/group-encryption.html) -- local-first group key management
- [Quarkslab: Secure Messaging Apps and Group Protocols](https://blog.quarkslab.com/secure-messaging-apps-and-group-protocols-part-2.html) -- MLS and group protocol analysis
- [Google A2A Protocol announcement](https://developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability/) -- agent-to-agent protocol landscape
- [World Economic Forum: AI agent safety (2025)](https://www.weforum.org/stories/2025/01/ai-agents-multi-agent-systems-safety/) -- agent autonomy safety
- [Strata: AI Agent Identity Crisis (2026)](https://www.strata.io/blog/agentic-identity/the-ai-agent-identity-crisis-new-research-reveals-a-governance-gap/) -- 80% of orgs lack real-time agent oversight
- [WebSocket Heartbeat Ping-Pong Best Practices (2026)](https://oneuptime.com/blog/post/2026-01-27-websocket-heartbeat-ping-pong/view) -- heartbeat configuration
- [Implementing E2E Encryption for Group Chats](https://medium.com/@asierr/implementing-end-to-end-encryption-for-group-chats-f068577c53de) -- group key management approaches
- [Meta: Building E2E Security for Messenger (2023)](https://engineering.fb.com/2023/12/06/security/building-end-to-end-security-for-messenger/) -- production E2E at scale

---
*Pitfalls research for: Pinch -- secure agent-to-agent encrypted messaging protocol*
*Researched: 2026-02-26*
