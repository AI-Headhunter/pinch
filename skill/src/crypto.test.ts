import { describe, it, expect, beforeAll } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import {
	ed25519PubToX25519,
	ed25519PrivToX25519,
	encrypt,
	decrypt,
	encryptWithNonce,
	ensureSodiumReady,
} from "./crypto.js";

interface IdentityVector {
	ed25519_seed: string;
	ed25519_public_key: string;
	ed25519_private_key: string;
	x25519_public_key: string;
	x25519_private_key: string;
	address: string;
}

interface CryptoVector {
	description: string;
	sender_ed25519_seed: string;
	recipient_ed25519_seed: string;
	sender_x25519_pub: string;
	sender_x25519_priv: string;
	recipient_x25519_pub: string;
	recipient_x25519_priv: string;
	nonce: string;
	plaintext: string;
	ciphertext: string;
}

function loadIdentityVectors(): IdentityVector[] {
	const data = readFileSync(
		resolve(__dirname, "../../testdata/identity_vectors.json"),
		"utf-8",
	);
	return JSON.parse(data).vectors;
}

function loadCryptoVectors(): CryptoVector[] {
	const data = readFileSync(
		resolve(__dirname, "../../testdata/crypto_vectors.json"),
		"utf-8",
	);
	return JSON.parse(data).vectors;
}

function hexToBytes(hex: string): Uint8Array {
	const bytes = new Uint8Array(hex.length / 2);
	for (let i = 0; i < hex.length; i += 2) {
		bytes[i / 2] = Number.parseInt(hex.substring(i, i + 2), 16);
	}
	return bytes;
}

function bytesToHex(bytes: Uint8Array): string {
	return Array.from(bytes)
		.map((b) => b.toString(16).padStart(2, "0"))
		.join("");
}

beforeAll(async () => {
	await ensureSodiumReady();
});

describe("Ed25519 to X25519 conversion", () => {
	it("converts public keys to match test vectors", () => {
		const vectors = loadIdentityVectors();
		for (const v of vectors) {
			const pubKey = hexToBytes(v.ed25519_public_key);
			const x25519Pub = ed25519PubToX25519(pubKey);
			expect(bytesToHex(x25519Pub)).toBe(v.x25519_public_key);
		}
	});

	it("converts private keys to match test vectors", () => {
		const vectors = loadIdentityVectors();
		for (const v of vectors) {
			const privKey = hexToBytes(v.ed25519_private_key);
			const x25519Priv = ed25519PrivToX25519(privKey);
			expect(bytesToHex(x25519Priv)).toBe(v.x25519_private_key);
		}
	});
});

describe("NaCl box encryption", () => {
	it("encrypts with known nonce to match test vectors", () => {
		const vectors = loadCryptoVectors();
		for (const v of vectors) {
			const recipPub = hexToBytes(v.recipient_x25519_pub);
			const senderPriv = hexToBytes(v.sender_x25519_priv);
			const nonce = hexToBytes(v.nonce);
			const plaintext = hexToBytes(v.plaintext);

			const sealed = encryptWithNonce(plaintext, recipPub, senderPriv, nonce);
			expect(bytesToHex(sealed)).toBe(v.ciphertext);
		}
	});

	it("decrypts known ciphertext to match test vectors", () => {
		const vectors = loadCryptoVectors();
		for (const v of vectors) {
			const senderPub = hexToBytes(v.sender_x25519_pub);
			const recipPriv = hexToBytes(v.recipient_x25519_priv);
			const ciphertext = hexToBytes(v.ciphertext);

			const decrypted = decrypt(ciphertext, senderPub, recipPriv);
			expect(bytesToHex(decrypted)).toBe(v.plaintext);
		}
	});

	it("produces different ciphertexts for same plaintext (random nonce)", async () => {
		const vectors = loadCryptoVectors();
		const v = vectors[0];
		const recipPub = hexToBytes(v.recipient_x25519_pub);
		const senderPriv = hexToBytes(v.sender_x25519_priv);
		const plaintext = hexToBytes(v.plaintext);

		const c1 = encrypt(plaintext, recipPub, senderPriv);
		const c2 = encrypt(plaintext, recipPub, senderPriv);

		expect(bytesToHex(c1)).not.toBe(bytesToHex(c2));
	});
});
