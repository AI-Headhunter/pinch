/**
 * NaCl box encryption/decryption and Ed25519-to-X25519 key conversion
 * for the Pinch protocol.
 *
 * IMPORTANT: Call ensureSodiumReady() before using any crypto functions.
 * Uses the libsodium-wrappers-sumo variant for Ed25519-to-Curve25519 conversion.
 */

import sodium from "libsodium-wrappers-sumo";

let sodiumReady = false;

/**
 * Ensures libsodium is initialized. Must be called before any crypto operations.
 * Safe to call multiple times (singleton initialization).
 */
export async function ensureSodiumReady(): Promise<void> {
	if (!sodiumReady) {
		await sodium.ready;
		sodiumReady = true;
	}
}

/**
 * Converts an Ed25519 public key to the equivalent X25519 (Curve25519) public key.
 * @param pubKey - 32-byte Ed25519 public key
 * @returns 32-byte X25519 public key
 */
export function ed25519PubToX25519(pubKey: Uint8Array): Uint8Array {
	return sodium.crypto_sign_ed25519_pk_to_curve25519(pubKey);
}

/**
 * Converts an Ed25519 private key (64 bytes: seed + public) to the equivalent
 * X25519 private key.
 * @param privKey - 64-byte Ed25519 private key
 * @returns 32-byte X25519 private key
 */
export function ed25519PrivToX25519(privKey: Uint8Array): Uint8Array {
	return sodium.crypto_sign_ed25519_sk_to_curve25519(privKey);
}

/**
 * Encrypts plaintext using NaCl box with a random 24-byte nonce.
 * The nonce is prepended to the ciphertext in the returned Uint8Array.
 * @param plaintext - Data to encrypt
 * @param recipientX25519Pub - Recipient's 32-byte X25519 public key
 * @param senderX25519Priv - Sender's 32-byte X25519 private key
 * @returns Nonce (24 bytes) + ciphertext
 */
export function encrypt(
	plaintext: Uint8Array,
	recipientX25519Pub: Uint8Array,
	senderX25519Priv: Uint8Array,
): Uint8Array {
	const nonce = sodium.randombytes_buf(sodium.crypto_box_NONCEBYTES);
	const ciphertext = sodium.crypto_box_easy(
		plaintext,
		nonce,
		recipientX25519Pub,
		senderX25519Priv,
	);
	const result = new Uint8Array(nonce.length + ciphertext.length);
	result.set(nonce);
	result.set(ciphertext, nonce.length);
	return result;
}

/**
 * Decrypts sealed bytes produced by encrypt(). Expects the first 24 bytes to
 * be the nonce, followed by the NaCl box ciphertext.
 * @param sealed - Nonce (24 bytes) + ciphertext
 * @param senderX25519Pub - Sender's 32-byte X25519 public key
 * @param recipientX25519Priv - Recipient's 32-byte X25519 private key
 * @returns Decrypted plaintext
 * @throws If decryption fails (authentication error)
 */
export function decrypt(
	sealed: Uint8Array,
	senderX25519Pub: Uint8Array,
	recipientX25519Priv: Uint8Array,
): Uint8Array {
	const nonce = sealed.slice(0, sodium.crypto_box_NONCEBYTES);
	const ciphertext = sealed.slice(sodium.crypto_box_NONCEBYTES);
	return sodium.crypto_box_open_easy(
		ciphertext,
		nonce,
		senderX25519Pub,
		recipientX25519Priv,
	);
}

/**
 * Encrypts plaintext with a specified nonce. For test vector validation only.
 * Production code should use encrypt() which generates a random nonce.
 * @param plaintext - Data to encrypt
 * @param recipientX25519Pub - Recipient's 32-byte X25519 public key
 * @param senderX25519Priv - Sender's 32-byte X25519 private key
 * @param nonce - 24-byte nonce
 * @returns Nonce (24 bytes) + ciphertext
 */
export function encryptWithNonce(
	plaintext: Uint8Array,
	recipientX25519Pub: Uint8Array,
	senderX25519Priv: Uint8Array,
	nonce: Uint8Array,
): Uint8Array {
	const ciphertext = sodium.crypto_box_easy(
		plaintext,
		nonce,
		recipientX25519Pub,
		senderX25519Priv,
	);
	const result = new Uint8Array(nonce.length + ciphertext.length);
	result.set(nonce);
	result.set(ciphertext, nonce.length);
	return result;
}
