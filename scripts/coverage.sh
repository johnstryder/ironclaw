#!/usr/bin/env bash
# coverage.sh — Run all tests and report coverage with failures highlighted.
# Usage: ./scripts/coverage.sh

set -euo pipefail

PROFILE="/tmp/ironclaw_coverage_$$.out"
TEST_LOG="/tmp/ironclaw_testlog_$$.txt"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

cleanup() { rm -f "$PROFILE" "$TEST_LOG"; }
trap cleanup EXIT

echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo -e "${BOLD}  IronClaw Test Coverage Report${RESET}"
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo ""

# ── 1. Run all tests ────────────────────────────────────────────────────────
echo -e "${BOLD}Running tests...${RESET}"
echo ""

CGO_ENABLED=0 go test -v -cover -coverprofile="$PROFILE" -count=1 ./cmd/... ./internal/... > "$TEST_LOG" 2>&1 || true

# ── 2. Collect failures ────────────────────────────────────────────────────
FAILURES=$(grep -E "^--- FAIL:" "$TEST_LOG" || true)
FAIL_PACKAGES=$(grep -P "^FAIL\t" "$TEST_LOG" || true)

# ── 3. Show package results ────────────────────────────────────────────────
echo -e "${BOLD}Package Results:${RESET}"
echo ""

while IFS= read -r line; do
    if echo "$line" | grep -q "^ok"; then
        pkg=$(echo "$line" | awk '{print $2}')
        cov=$(echo "$line" | grep -oP 'coverage: \K[0-9.]+')
        if [ "$cov" = "100.0" ]; then
            printf "  ${GREEN}✓${RESET} %-50s ${GREEN}%s%%${RESET}\n" "$pkg" "$cov"
        elif [ -n "$cov" ]; then
            printf "  ${YELLOW}△${RESET} %-50s ${YELLOW}%s%%${RESET}\n" "$pkg" "$cov"
        fi
    elif echo "$line" | grep -q "^FAIL"; then
        pkg=$(echo "$line" | awk '{print $2}')
        printf "  ${RED}✗${RESET} %-50s ${RED}FAIL${RESET}\n" "$pkg"
    fi
done < "$TEST_LOG"

echo ""

# ── 4. Show failures in detail ─────────────────────────────────────────────
if [ -n "$FAILURES" ]; then
    echo -e "${BOLD}${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo -e "${BOLD}${RED}  FAILING TESTS${RESET}"
    echo -e "${BOLD}${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo ""
    awk '
        /^=== RUN/ { test_name=$0; capture=0; block="" }
        /^--- FAIL:/ { capture=1; print test_name; print $0; next }
        capture && /^=== RUN|^--- (PASS|FAIL)|^PASS|^FAIL|^ok/ { capture=0 }
        capture { print "    " $0 }
    ' "$TEST_LOG"
    echo ""
fi

# ── 5. Show non-100% functions ─────────────────────────────────────────────
if [ -f "$PROFILE" ]; then
    NON_FULL=$(go tool cover -func="$PROFILE" 2>/dev/null | grep -v "100.0%" | grep -v "^total:" || true)

    if [ -n "$NON_FULL" ]; then
        echo -e "${BOLD}${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
        echo -e "${BOLD}${YELLOW}  Functions Below 100% Coverage${RESET}"
        echo -e "${BOLD}${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
        echo ""
        echo "$NON_FULL" | while IFS= read -r line; do
            echo -e "  ${YELLOW}△${RESET} $line"
        done
        echo ""
    fi

    TOTAL=$(go tool cover -func="$PROFILE" 2>/dev/null | grep "^total:" | awk '{print $NF}')
else
    TOTAL="N/A"
fi

# ── 6. Summary ──────────────────────────────────────────────────────────────
PASS_COUNT=$(grep -c "^--- PASS:" "$TEST_LOG" || true)
FAIL_COUNT=$(grep -c "^--- FAIL:" "$TEST_LOG" || true)

echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo -e "${BOLD}  Summary${RESET}"
echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo ""
echo -e "  Tests passed:  ${GREEN}${PASS_COUNT}${RESET}"

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo -e "  Tests failed:  ${RED}${FAIL_COUNT}${RESET}"
else
    echo -e "  Tests failed:  ${GREEN}0${RESET}"
fi

if [ "$TOTAL" = "100.0%" ]; then
    echo -e "  Coverage:      ${GREEN}${TOTAL}${RESET}"
else
    echo -e "  Coverage:      ${YELLOW}${TOTAL}${RESET}"
fi

echo ""

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo -e "  ${RED}${BOLD}BUILD: FAILED${RESET}"
    exit 1
else
    echo -e "  ${GREEN}${BOLD}BUILD: PASSED${RESET}"
fi
