#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

PROTO_JS_PATH="gen/ts/pinch/v1/envelope_pb.js"
if [[ ! -f "$PROTO_JS_PATH" ]]; then
	echo "FAIL: Missing $PROTO_JS_PATH. Run \`pnpm run build\` first." >&2
	exit 1
fi

TMP_DIR="$(mktemp -d)"
ERR_FILE="$TMP_DIR/pinch-contacts.stderr"
trap 'rm -rf "$TMP_DIR"' EXIT

set +e
PINCH_RELAY_URL="ws://127.0.0.1:9/ws" \
PINCH_RELAY_HOST="127.0.0.1" \
PINCH_KEYPAIR_PATH="$TMP_DIR/keypair.json" \
PINCH_DATA_DIR="$TMP_DIR/data" \
pnpm --dir skill exec pinch-contacts > /dev/null 2>"$ERR_FILE"
STATUS=$?
set -e

OUTPUT="$(cat "$ERR_FILE")"

if [[ "$OUTPUT" == *"ERR_MODULE_NOT_FOUND"* && "$OUTPUT" == *"envelope_pb.js"* ]]; then
	echo "FAIL: pinch-contacts failed to load generated proto JS output." >&2
	echo "$OUTPUT" >&2
	exit 1
fi

if [[ "$OUTPUT" == *"Could not locate the bindings file"* && "$OUTPUT" == *"better_sqlite3.node"* ]]; then
	echo "FAIL: pinch-contacts failed to load better-sqlite3 native bindings." >&2
	echo "$OUTPUT" >&2
	exit 1
fi

if [[ "$STATUS" -ne 0 ]]; then
	echo "INFO: pinch-contacts exited non-zero without a reachable relay (expected in smoke mode)." >&2
fi

echo "PASS: install/build smoke checks passed (proto JS + sqlite bindings load)."
