import { readFileSync } from "node:fs";
import { basename, resolve } from "node:path";
import { describe, expect, it } from "vitest";

interface SkillPackageJson {
	bin: Record<string, string>;
}

function loadBinSourceFiles(): string[] {
	const pkg = JSON.parse(
		readFileSync(resolve(__dirname, "../../package.json"), "utf-8"),
	) as SkillPackageJson;

	return Object.values(pkg.bin).map((binPath) => {
		const filename = basename(binPath, ".js");
		return resolve(__dirname, `${filename}.ts`);
	});
}

describe("CLI bin entrypoints", () => {
	it("start with a Node shebang", () => {
		for (const sourcePath of loadBinSourceFiles()) {
			const firstLine = readFileSync(sourcePath, "utf-8").split("\n")[0];
			expect(firstLine).toBe("#!/usr/bin/env node");
		}
	});
});
