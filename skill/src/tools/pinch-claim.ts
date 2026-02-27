/**
 * pinch-claim -- Approve a pending agent registration (operator tool).
 *
 * Usage:
 *   pinch-claim <CLAIM_CODE>
 *
 * Environment variables:
 *   PINCH_RELAY_URL          WebSocket URL of the relay server (required)
 *   PINCH_RELAY_ADMIN_SECRET Admin secret to authenticate the claim (required)
 *
 * Example:
 *   PINCH_RELAY_URL=wss://relay.example.com/ws \
 *   PINCH_RELAY_ADMIN_SECRET=supersecret \
 *   pinch-claim DEAD1234
 */

import { relayBaseUrl } from "./relay-url.js";

/** Execute the pinch-claim tool. */
export async function run(args: string[]): Promise<void> {
	const claimCode = args[0];
	if (!claimCode) {
		console.error("Usage: pinch-claim <CLAIM_CODE>");
		process.exit(1);
	}

	const relayUrl = process.env.PINCH_RELAY_URL;
	if (!relayUrl) {
		console.error("Error: PINCH_RELAY_URL environment variable is required");
		process.exit(1);
	}

	const adminSecret = process.env.PINCH_RELAY_ADMIN_SECRET;
	if (!adminSecret) {
		console.error("Error: PINCH_RELAY_ADMIN_SECRET environment variable is required");
		process.exit(1);
	}

	const baseUrl = relayBaseUrl(relayUrl);

	const response = await fetch(`${baseUrl}/agents/claim`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ claim_code: claimCode, admin_secret: adminSecret }),
	});

	if (!response.ok) {
		const text = await response.text();
		console.error(`Error: claim failed (${response.status}): ${text.trim()}`);
		process.exit(1);
	}

	const result = (await response.json()) as { address: string; status: string };
	console.log(`Approved: ${result.address}`);
}

// Self-executable entry point.
if (
	process.argv[1] &&
	(process.argv[1].endsWith("pinch-claim.ts") ||
		process.argv[1].endsWith("pinch-claim.js"))
) {
	run(process.argv.slice(2)).catch((err) => {
		console.error(`Error: ${String(err.message ?? err)}`);
		process.exit(1);
	});
}
