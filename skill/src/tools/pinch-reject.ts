/**
 * pinch_reject -- Silently reject a pending inbound connection request.
 *
 * No network message is sent to the requester (silent rejection per protocol).
 * Transitions the connection state from pending_inbound → revoked locally
 * and saves the store.
 *
 * Usage:
 *   pinch-reject --connection <address>
 *
 * Outputs JSON: { "status": "rejected", "connection": "<address>" }
 */

import { bootstrap, shutdown } from "./cli.js";

/** Parsed arguments for pinch_reject. */
export interface RejectArgs {
	connection: string;
}

/** Parse CLI arguments into a structured object. */
export function parseArgs(args: string[]): RejectArgs {
	let connection = "";

	for (let i = 0; i < args.length; i++) {
		switch (args[i]) {
			case "--connection":
				connection = args[++i] ?? "";
				break;
		}
	}

	if (!connection) throw new Error("--connection is required");

	return { connection };
}

/** Execute the pinch_reject tool. */
export async function run(args: string[]): Promise<void> {
	const parsed = parseArgs(args);
	const { connectionManager } = await bootstrap();

	await connectionManager.rejectRequest(parsed.connection);

	console.log(
		JSON.stringify({
			status: "rejected",
			connection: parsed.connection,
		}),
	);

	await shutdown();
}

// Self-executable entry point.
if (
	process.argv[1] &&
	(process.argv[1].endsWith("pinch-reject.ts") ||
		process.argv[1].endsWith("pinch-reject.js"))
) {
	run(process.argv.slice(2)).catch((err) => {
		console.error(JSON.stringify({ error: String(err.message ?? err) }));
		process.exit(1);
	});
}
