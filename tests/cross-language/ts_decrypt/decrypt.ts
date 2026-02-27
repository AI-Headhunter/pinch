/**
 * TypeScript decrypt program for cross-language integration tests.
 * Reads JSON from stdin: {ed25519_seed_sender, ed25519_seed_recipient, sealed}
 * Outputs JSON to stdout: {plaintext}
 */

import sodium from "libsodium-wrappers-sumo";
import { ed25519PubToX25519, ed25519PrivToX25519, decrypt, ensureSodiumReady } from "../../../skill/src/crypto.ts";

function hexToBytes(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.substring(i, i + 2), 16);
  }
  return bytes;
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

async function main() {
  await sodium.ready;
  await ensureSodiumReady();

  const chunks: Buffer[] = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk);
  }
  const input = JSON.parse(Buffer.concat(chunks).toString("utf-8"));

  const senderSeed = hexToBytes(input.ed25519_seed_sender);
  const recipientSeed = hexToBytes(input.ed25519_seed_recipient);
  const sealed = hexToBytes(input.sealed);

  // Generate Ed25519 keypairs from seeds
  const senderKp = sodium.crypto_sign_seed_keypair(senderSeed);
  const recipientKp = sodium.crypto_sign_seed_keypair(recipientSeed);

  // Convert to X25519
  const senderX25519Pub = ed25519PubToX25519(senderKp.publicKey);
  const recipientX25519Priv = ed25519PrivToX25519(recipientKp.privateKey);

  // Decrypt
  const plaintext = decrypt(sealed, senderX25519Pub, recipientX25519Priv);

  console.log(JSON.stringify({ plaintext: bytesToHex(plaintext) }));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
