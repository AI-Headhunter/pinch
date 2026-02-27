/**
 * pinch_accept -- Approve a pending inbound connection request.
 *
 * Sends a ConnectionResponse protobuf (accepted=true) over WebSocket,
 * transitions the connection state from pending_inbound → active,
 * and saves the store.
 *
 * Usage:
 *   pinch-accept --connection <address>
 *
 * Outputs JSON: { "status": "accepted", "connection": "<address>" }
 */

import { bootstrap, shutdown } from "./cli.js";

/** Parsed arguments for pinch_accept. */
export interface AcceptArgs {
	connection: string;
}

/** Parse CLI arguments into a structured object. */
export function parseArgs(args: string[]): AcceptArgs {
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

/** Execute the pinch_accept tool. */
export async function run(args: string[]): Promise<void> {
	const parsed = parseArgs(args);
	const { connectionManager } = await bootstrap();

	await connectionManager.approveRequest(parsed.connection);

	console.log(
		JSON.stringify({
			status: "accepted",
			connection: parsed.connection,
		}),
	);

	await shutdown();
}

// Self-executable entry point.
if (
	process.argv[1] &&
	(process.argv[1].endsWith("pinch-accept.ts") ||
		process.argv[1].endsWith("pinch-accept.js"))
) {
	run(process.argv.slice(2)).catch((err) => {
		console.error(JSON.stringify({ error: String(err.message ?? err) }));
		process.exit(1);
	});
}
