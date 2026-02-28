import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const BIN_TOOL_FILES = new Set([
	"pinch-send.ts",
	"pinch-connect.ts",
	"pinch-accept.ts",
	"pinch-reject.ts",
	"pinch-contacts.ts",
	"pinch-history.ts",
	"pinch-status.ts",
	"pinch-autonomy.ts",
	"pinch-permissions.ts",
	"pinch-activity.ts",
	"pinch-intervene.ts",
	"pinch-mute.ts",
	"pinch-audit-verify.ts",
	"pinch-audit-export.ts",
	"pinch-whoami.ts",
	"pinch-claim.ts",
]);

describe("CLI entrypoint shebangs", () => {
	it("adds a node shebang to every published bin tool", () => {
		const toolsDir = __dirname;
		const actual = readdirSync(toolsDir).filter((file) =>
			BIN_TOOL_FILES.has(file),
		);

		expect(actual.sort()).toEqual(Array.from(BIN_TOOL_FILES).sort());

		for (const file of actual) {
			const firstLine = readFileSync(join(toolsDir, file), "utf-8").split(
				"\n",
			)[0];
			expect(firstLine).toBe("#!/usr/bin/env node");
		}
	});
});
