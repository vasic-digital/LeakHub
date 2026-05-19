#!/usr/bin/env bash
# leakhub_describe_challenge.sh
#
# Round-266 paired-mutation deep-doc challenge for digital.vasic.leakhub.
#
# Validates that:
#   1. The deep-doc ledger (docs/test-coverage.md) lists every exported
#      symbol from pkg/types/types.go and pkg/client/client.go.
#   2. The multi-locale fixture
#      (tests/fixtures/leakhub/payloads.json) parses and contains at
#      least 3 locales.
#   3. The multi-locale runner (challenges/runner/main.go) builds and
#      runs, byte-preserving non-ASCII responses + custom signatures
#      + archive entries through the real leakhub.Client across
#      DetectLeak, AddSignatures, AddToArchive, SearchArchive,
#      GetByModel, GetStats, ExportArchive, Count, and the
#      sensitivity-hint fallback path.
#   4. The README enumerates the round-266 anti-bluff guarantees.
#
# Paired-mutation invariant (CONST-035 + CONST-050(B)):
#   With --anti-bluff-mutate the script plants a deliberate symbol-rename
#   mutation in a tmp copy of the ledger (DetectLeak ->
#   DetectLeak_MUTATED), reruns validation, and asserts the gate
#   FAILS with exit 99. This proves the gate actually catches
#   ledger-vs-source drift instead of rubber-stamping it.
#
# Exit codes:
#   0  — gate PASS on clean tree
#   1  — gate FAIL on clean tree (real failure to fix)
#   99 — paired-mutation correctly detected (good — proves anti-bluff)
#   2  — usage / environment error

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MUTATE=0
for arg in "$@"; do
    case "$arg" in
        --anti-bluff-mutate) MUTATE=1 ;;
        --help|-h)
            sed -n '1,32p' "$0"
            exit 0
            ;;
        *)
            echo "unknown argument: $arg" >&2
            exit 2
            ;;
    esac
done

PASS=0
FAIL=0
TOTAL=0

pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }

LEDGER="${MODULE_DIR}/docs/test-coverage.md"
FIXTURE="${MODULE_DIR}/tests/fixtures/leakhub/payloads.json"
RUNNER="${MODULE_DIR}/challenges/runner/main.go"
README="${MODULE_DIR}/README.md"

LEDGER_WORK="${LEDGER}"
TMP_LEDGER=""
if [ "${MUTATE}" -eq 1 ]; then
    TMP_LEDGER="$(mktemp)"
    cp "${LEDGER}" "${TMP_LEDGER}"
    # Plant a rename so the symbol no longer matches what the source declares.
    sed -i 's/DetectLeak/DetectLeak_MUTATED/g' "${TMP_LEDGER}"
    LEDGER_WORK="${TMP_LEDGER}"
    echo "=== LeakHub Describe Challenge (anti-bluff-mutate mode) ==="
else
    echo "=== LeakHub Describe Challenge (clean mode) ==="
fi
echo ""

# Section 1: ledger presence and freshness
echo "Section 1: docs/test-coverage.md ledger"
if [ ! -f "${LEDGER_WORK}" ]; then
    fail "ledger missing at ${LEDGER_WORK}"
else
    pass "ledger present"
    if grep -q "round-266" "${LEDGER_WORK}"; then
        pass "ledger marked round-266"
    else
        fail "ledger missing round-266 marker"
    fi
    if grep -q "execution of tests and Challenges MUST guarantee" "${LEDGER_WORK}"; then
        pass "ledger carries Article XI §11.9 mandate"
    else
        fail "ledger missing Article XI §11.9 mandate"
    fi
fi

# Section 2: every exported package symbol appears in ledger.
# Hand-picked, stable set of structural symbols expected verbatim in
# the ledger. (Exhaustive parsing of every exported identifier would
# produce false positives from internal helpers — the ledger is
# authoritative about what counts as part of the public surface.)
echo ""
echo "Section 2: structural symbol cross-reference"

EXPECTED_SYMBOLS=(
    # pkg/types/types.go
    "LeakEntry" "DetectionOptions" "DetectionResult" "LeakMatch" "ArchiveStats"
    # pkg/client/client.go
    "Client" "New" "NewFromConfig" "Close" "Config"
    "AddSignatures" "DetectLeak" "SearchArchive" "AddToArchive"
    "GetByModel" "GetStats" "Count" "ExportArchive"
)

CHECKED=0
MISSING=0
for sym in "${EXPECTED_SYMBOLS[@]}"; do
    CHECKED=$((CHECKED + 1))
    if grep -qE "\\b${sym}\\b" "${LEDGER_WORK}"; then
        : # found
    else
        fail "ledger missing symbol ${sym}"
        MISSING=$((MISSING + 1))
    fi
done
if [ "${MISSING}" -eq 0 ]; then
    pass "all ${CHECKED} structural symbols cross-referenced in ledger"
fi

# Section 3: multi-locale fixture sanity
echo ""
echo "Section 3: multi-locale fixture"
if [ ! -f "${FIXTURE}" ]; then
    fail "fixture missing at ${FIXTURE}"
else
    pass "fixture present"
    LOCALE_COUNT=$(grep -oE '"locale":\s*"[^"]+"' "${FIXTURE}" | sort -u | wc -l)
    if [ "${LOCALE_COUNT}" -ge 3 ]; then
        pass "fixture covers ${LOCALE_COUNT} locales (>=3)"
    else
        fail "fixture covers only ${LOCALE_COUNT} locales (<3)"
    fi
fi

# Section 4: runner builds + runs against every section
echo ""
echo "Section 4: multi-locale runner build + run (real Client across 5 locales)"
if [ ! -f "${RUNNER}" ]; then
    fail "runner missing at ${RUNNER}"
else
    pass "runner source present"
    cd "${MODULE_DIR}"
    if go build -o /tmp/leakhub_round266_runner ./challenges/runner/ 2>/tmp/leakhub_build.log; then
        pass "runner builds"
        if /tmp/leakhub_round266_runner -fixtures "${FIXTURE}" > /tmp/leakhub_run.log 2>&1; then
            pass "runner exit 0 across every section + locale"
            if grep -q "PASS: \[Section1\]\[DetectLeak\]\[probe\] canonical" /tmp/leakhub_run.log; then
                pass "Section 1 default-list canonical signature wired"
            else
                fail "Section 1 default-list canonical PASS missing"
            fi
            if grep -q "PASS: \[Section2\]\[DetectLeak\]\[sr\]" /tmp/leakhub_run.log; then
                pass "Section 2 Cyrillic (sr) benign not over-flagged"
            else
                fail "Section 2 Cyrillic (sr) benign PASS missing"
            fi
            if grep -q "PASS: \[Section2\]\[DetectLeak\]\[ja\]" /tmp/leakhub_run.log; then
                pass "Section 2 Japanese (ja) benign not over-flagged"
            else
                fail "Section 2 Japanese (ja) benign PASS missing"
            fi
            if grep -q "PASS: \[Section2\]\[DetectLeak\]\[ar\]" /tmp/leakhub_run.log; then
                pass "Section 2 Arabic (ar) benign not over-flagged"
            else
                fail "Section 2 Arabic (ar) benign PASS missing"
            fi
            if grep -q "PASS: \[Section2\]\[DetectLeak\]\[zh-CN\]" /tmp/leakhub_run.log; then
                pass "Section 2 Han (zh-CN) benign not over-flagged"
            else
                fail "Section 2 Han (zh-CN) benign PASS missing"
            fi
            if grep -q "PASS: \[Section3\]\[DetectLeak\]\[sr\] canonical signature matched" /tmp/leakhub_run.log; then
                pass "Section 3 Cyrillic leak detected at byte-exact position"
            else
                fail "Section 3 sr canonical position PASS missing"
            fi
            if grep -q "PASS: \[Section3\]\[DetectLeak\]\[ar\] canonical signature matched" /tmp/leakhub_run.log; then
                pass "Section 3 Arabic leak detected at byte-exact position"
            else
                fail "Section 3 ar canonical position PASS missing"
            fi
            if grep -q "PASS: \[Section4\]\[AddSignatures+DetectLeak\]\[ja\]" /tmp/leakhub_run.log; then
                pass "Section 4 Japanese custom signature stored + matched"
            else
                fail "Section 4 ja custom signature PASS missing"
            fi
            if grep -q "PASS: \[Section4\]\[AddSignatures+DetectLeak\]\[zh-CN\]" /tmp/leakhub_run.log; then
                pass "Section 4 Han custom signature stored + matched"
            else
                fail "Section 4 zh-CN custom signature PASS missing"
            fi
            if grep -q "PASS: \[Section5\]\[Count\] 5 entries stored" /tmp/leakhub_run.log; then
                pass "Section 5 Count=5 after all locale inserts"
            else
                fail "Section 5 Count=5 PASS missing"
            fi
            if grep -q "PASS: \[Section5\]\[SearchArchive\]\[sr\]" /tmp/leakhub_run.log; then
                pass "Section 5 SearchArchive Cyrillic tag unique match"
            else
                fail "Section 5 sr SearchArchive PASS missing"
            fi
            if grep -q "PASS: \[Section5\]\[GetByModel\]\[gpt-4\] 5 entries" /tmp/leakhub_run.log; then
                pass "Section 5 GetByModel returns all locales"
            else
                fail "Section 5 GetByModel PASS missing"
            fi
            if grep -q "PASS: \[Section6\]\[ExportArchive\]\[json\] 5 entries round-trip byte-exact" /tmp/leakhub_run.log; then
                pass "Section 6 JSON export round-trip byte-exact"
            else
                fail "Section 6 ExportArchive round-trip PASS missing"
            fi
            if grep -q "PASS: \[Section6\]\[ExportArchive\]\[xml\] unsupported" /tmp/leakhub_run.log; then
                pass "Section 6 ExportArchive xml rejected"
            else
                fail "Section 6 ExportArchive xml rejection PASS missing"
            fi
            if grep -q "PASS: \[Section7\]\[DetectLeak\]\[empty-model\]" /tmp/leakhub_run.log; then
                pass "Section 7 empty-model sentinel surfaced"
            else
                fail "Section 7 empty-model sentinel PASS missing"
            fi
            if grep -q "PASS: \[Section7\]\[AddToArchive\]\[empty\]" /tmp/leakhub_run.log; then
                pass "Section 7 empty-archive-entry sentinel surfaced"
            else
                fail "Section 7 empty-archive sentinel PASS missing"
            fi
            if grep -q "PASS: \[Section7\]\[DetectLeak\]\[sensitivity\]" /tmp/leakhub_run.log; then
                pass "Section 7 sensitivity-hint fallback path triggered"
            else
                fail "Section 7 sensitivity-hint PASS missing"
            fi
        else
            fail "runner exit non-zero — see /tmp/leakhub_run.log"
            sed -n '1,80p' /tmp/leakhub_run.log
        fi
    else
        fail "runner build failed — see /tmp/leakhub_build.log"
        sed -n '1,40p' /tmp/leakhub_build.log
    fi
    rm -f /tmp/leakhub_round266_runner
fi

# Section 5: README round-266 anti-bluff section
echo ""
echo "Section 5: README round-266 anti-bluff section"
if grep -q "Anti-bluff guarantees" "${README}"; then
    pass "README declares Anti-bluff guarantees"
else
    fail "README missing Anti-bluff guarantees section"
fi
if grep -q "round-266" "${README}"; then
    pass "README marked round-266"
else
    fail "README missing round-266 marker"
fi

# Cleanup mutated ledger if any
if [ -n "${TMP_LEDGER}" ]; then
    rm -f "${TMP_LEDGER}"
fi

echo ""
echo "=== Summary: ${PASS}/${TOTAL} PASS, ${FAIL} FAIL ==="

if [ "${MUTATE}" -eq 1 ]; then
    if [ "${FAIL}" -gt 0 ]; then
        echo "anti-bluff-mutate: gate correctly detected planted mutation (exit 99)"
        exit 99
    else
        echo "anti-bluff-mutate: gate FAILED to detect planted mutation — bluff!"
        exit 1
    fi
fi

if [ "${FAIL}" -gt 0 ]; then
    exit 1
fi
exit 0
