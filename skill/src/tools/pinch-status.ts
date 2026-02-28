#!/usr/bin/env node
/**
 * pinch_status -- Return delivery state for a message_id.
 *
 * Usage:
 *   pinch-status --id <message_id>
 *
 * If found: { "message_id": "<id>", "state": "<state>", "failure_reason": "<reason or null>", "updated_at": "<timestamp>" }
 * If not found: { "error": "message not found" } (exit 1)
 */

import { bootstrap, runToolEntrypoint, shutdown } from "./cli.js";

/** Parse CLI arguments into a structured object. */
export function parseArgs(args: string[]): {
	id: string;
} {
	let id = "";

	for (let i = 0; i < args.length; i++) {
		if (args[i] === "--id") {
			id = args[++i] ?? "";
		}
	}

	if (!id) throw new Error("--id is required");

	return { id };
}

/** Execute the pinch_status tool. */
export async function run(args: string[]): Promise<void> {
	const parsed = parseArgs(args);
	const { messageStore } = await bootstrap();

	const message = messageStore.getMessage(parsed.id);

	if (!message) {
		console.log(JSON.stringify({ error: "message not found" }));
		await shutdown();
		process.exit(1);
	}

	console.log(
		JSON.stringify({
			message_id: message.id,
			state: message.state,
			failure_reason: message.failureReason ?? null,
			updated_at: message.updatedAt,
		}),
	);
	await shutdown();
}

runToolEntrypoint("pinch-status", run);
