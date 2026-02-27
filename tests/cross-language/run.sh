#!/bin/bash
set -euo pipefail

# Cross-language crypto integration test
# Tests that Go and TypeScript produce interoperable NaCl box encryption.
#
# Flow:
# 1. Go encrypts -> TypeScript decrypts
# 2. TypeScript encrypts -> Go decrypts

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$SCRIPT_DIR/.build"
mkdir -p "$BUILD_DIR"

# Build Go programs (inside relay module to access internal packages)
echo "Building Go encrypt/decrypt programs..."
(cd "$ROOT_DIR" && go build -o "$BUILD_DIR/go_encrypt" ./relay/cmd/crosstest-encrypt)
(cd "$ROOT_DIR" && go build -o "$BUILD_DIR/go_decrypt" ./relay/cmd/crosstest-decrypt)

TSX="$ROOT_DIR/skill/node_modules/.bin/tsx"

PASSED=0
FAILED=0
TOTAL=0

run_test() {
  local description="$1"
  local sender_seed="$2"
  local recipient_seed="$3"
  local plaintext_hex="$4"

  TOTAL=$((TOTAL + 1))

  # Test 1: Go encrypts -> TypeScript decrypts
  echo -n "  Go->TS ($description): "
  local go_sealed
  go_sealed=$(echo "{\"ed25519_seed_sender\":\"$sender_seed\",\"ed25519_seed_recipient\":\"$recipient_seed\",\"plaintext\":\"$plaintext_hex\"}" | "$BUILD_DIR/go_encrypt")
  local sealed_hex
  sealed_hex=$(echo "$go_sealed" | python3 -c "import sys,json; print(json.load(sys.stdin)['sealed'])")

  local ts_result
  ts_result=$(echo "{\"ed25519_seed_sender\":\"$sender_seed\",\"ed25519_seed_recipient\":\"$recipient_seed\",\"sealed\":\"$sealed_hex\"}" | "$TSX" "$SCRIPT_DIR/ts_decrypt/decrypt.ts")
  local decrypted_hex
  decrypted_hex=$(echo "$ts_result" | python3 -c "import sys,json; print(json.load(sys.stdin)['plaintext'])")

  if [ "$decrypted_hex" = "$plaintext_hex" ]; then
    echo "PASS"
    PASSED=$((PASSED + 1))
  else
    echo "FAIL (expected $plaintext_hex, got $decrypted_hex)"
    FAILED=$((FAILED + 1))
  fi

  TOTAL=$((TOTAL + 1))

  # Test 2: TypeScript encrypts -> Go decrypts
  echo -n "  TS->Go ($description): "
  local ts_sealed
  ts_sealed=$(echo "{\"ed25519_seed_sender\":\"$sender_seed\",\"ed25519_seed_recipient\":\"$recipient_seed\",\"plaintext\":\"$plaintext_hex\"}" | "$TSX" "$SCRIPT_DIR/ts_encrypt/encrypt.ts")
  sealed_hex=$(echo "$ts_sealed" | python3 -c "import sys,json; print(json.load(sys.stdin)['sealed'])")

  local go_result
  go_result=$(echo "{\"ed25519_seed_sender\":\"$sender_seed\",\"ed25519_seed_recipient\":\"$recipient_seed\",\"sealed\":\"$sealed_hex\"}" | "$BUILD_DIR/go_decrypt")
  decrypted_hex=$(echo "$go_result" | python3 -c "import sys,json; print(json.load(sys.stdin)['plaintext'])")

  if [ "$decrypted_hex" = "$plaintext_hex" ]; then
    echo "PASS"
    PASSED=$((PASSED + 1))
  else
    echo "FAIL (expected $plaintext_hex, got $decrypted_hex)"
    FAILED=$((FAILED + 1))
  fi
}

echo ""
echo "=== Cross-Language Crypto Integration Tests ==="
echo ""

# Test case 1: Basic text
SENDER_SEED_1="0000000000000000000000000000000000000000000000000000000000000001"
RECIP_SEED_1="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
PLAINTEXT_1="48656c6c6f2c2050696e636821"  # "Hello, Pinch!"

echo "Test 1: Basic text encryption"
run_test "Hello, Pinch!" "$SENDER_SEED_1" "$RECIP_SEED_1" "$PLAINTEXT_1"

# Test case 2: Longer text
SENDER_SEED_2="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
RECIP_SEED_2="deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
PLAINTEXT_2="43726f73732d6c616e67756167652063727970746f207465737420766563746f72"  # "Cross-language crypto test vector"

echo "Test 2: Longer text encryption"
run_test "Cross-language crypto test vector" "$SENDER_SEED_2" "$RECIP_SEED_2" "$PLAINTEXT_2"

# Test case 3: Binary data
SENDER_SEED_3="0000000000000000000000000000000000000000000000000000000000000001"
RECIP_SEED_3="deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
PLAINTEXT_3="deadbeef01020304"

echo "Test 3: Binary data encryption"
run_test "Binary data" "$SENDER_SEED_3" "$RECIP_SEED_3" "$PLAINTEXT_3"

echo ""
echo "=== Results: $PASSED passed, $FAILED failed, $TOTAL total ==="

# Clean up build artifacts
rm -rf "$BUILD_DIR"

if [ "$FAILED" -gt 0 ]; then
  exit 1
fi

echo "All cross-language crypto tests passed!"
exit 0
