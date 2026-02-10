#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
COMPARE_SCRIPT="$ROOT_DIR/benchmark/compare_pslog_commits_ab.sh"

BASE_COMMIT=${BASE_COMMIT:-}
CANDIDATE_COMMIT=${CANDIDATE_COMMIT:-HEAD}
RUNS=${RUNS:-6}
BENCHTIME=${BENCHTIME:-700ms}
GOMAXPROCS_VALUE=${GOMAXPROCS_VALUE:-1}
PIN_CORE=${PIN_CORE:-}
OUTPUT_DIR=${OUTPUT_DIR:-results/commit_ab}
STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
OUT_PREFIX=${OUT_PREFIX:-perf_gate_${STAMP}}
MAX_MEDIAN_DELTA_PERCENT=${MAX_MEDIAN_DELTA_PERCENT:-1.00}
CORE_BENCH_REGEX=${CORE_BENCH_REGEX:-^BenchmarkPSLogProduction/pslog/production/}
SKIP_GO_TEST=${SKIP_GO_TEST:-0}
SKIP_MAKE_CHECK=${SKIP_MAKE_CHECK:-0}

if [[ -z "$PIN_CORE" ]] && command -v taskset >/dev/null 2>&1; then
	PIN_CORE=$(( $(getconf _NPROCESSORS_ONLN) - 1 ))
fi

if [[ -z "$BASE_COMMIT" ]]; then
	echo "BASE_COMMIT is required (example: BASE_COMMIT=97012d4)." >&2
	exit 1
fi

if [[ ! "$RUNS" =~ ^[1-9][0-9]*$ ]]; then
	echo "RUNS must be a positive integer, got: $RUNS" >&2
	exit 1
fi

if [[ ! "$MAX_MEDIAN_DELTA_PERCENT" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
	echo "MAX_MEDIAN_DELTA_PERCENT must be a non-negative number, got: $MAX_MEDIAN_DELTA_PERCENT" >&2
	exit 1
fi

if [[ -n "$PIN_CORE" && ! "$PIN_CORE" =~ ^[0-9]+$ ]]; then
	echo "PIN_CORE must be a non-negative integer core index, got: $PIN_CORE" >&2
	exit 1
fi

if [[ -n "$PIN_CORE" ]] && ! command -v taskset >/dev/null 2>&1; then
	echo "PIN_CORE was set but taskset is unavailable on this host." >&2
	exit 1
fi

if [[ "$SKIP_GO_TEST" != "0" && "$SKIP_GO_TEST" != "1" ]]; then
	echo "SKIP_GO_TEST must be 0 or 1, got: $SKIP_GO_TEST" >&2
	exit 1
fi

if [[ "$SKIP_MAKE_CHECK" != "0" && "$SKIP_MAKE_CHECK" != "1" ]]; then
	echo "SKIP_MAKE_CHECK must be 0 or 1, got: $SKIP_MAKE_CHECK" >&2
	exit 1
fi

echo "Performance gate:"
echo "  base:               $BASE_COMMIT"
echo "  candidate:          $CANDIDATE_COMMIT"
echo "  runs:               $RUNS"
echo "  benchtime:          $BENCHTIME"
if [[ -n "$PIN_CORE" ]]; then
	echo "  pin core:           $PIN_CORE"
else
	echo "  pin core:           disabled"
fi
echo "  max median delta:   +${MAX_MEDIAN_DELTA_PERCENT}%"
echo "  core bench filter:  $CORE_BENCH_REGEX"
echo "  output prefix:      $OUT_PREFIX"
echo

if [[ "$SKIP_GO_TEST" == "0" ]]; then
	echo "[1/3] go test ./..."
	(
		cd "$ROOT_DIR"
		go test ./...
	)
else
	echo "[1/3] go test ./... (skipped)"
fi

if [[ "$SKIP_MAKE_CHECK" == "0" ]]; then
	echo "[2/3] make check"
	(
		cd "$ROOT_DIR"
		make check
	)
else
	echo "[2/3] make check (skipped)"
fi

echo "[3/3] commit A/B benchmark comparison"
(
	cd "$ROOT_DIR"
	BASE_COMMIT="$BASE_COMMIT" \
	CANDIDATE_COMMIT="$CANDIDATE_COMMIT" \
	RUNS="$RUNS" \
	BENCHTIME="$BENCHTIME" \
	GOMAXPROCS_VALUE="$GOMAXPROCS_VALUE" \
	PIN_CORE="$PIN_CORE" \
	OUTPUT_DIR="$OUTPUT_DIR" \
	OUT_PREFIX="$OUT_PREFIX" \
	"$COMPARE_SCRIPT"
)

summary_csv="$ROOT_DIR/$OUTPUT_DIR/${OUT_PREFIX}_summary.csv"
if [[ ! -f "$summary_csv" ]]; then
	echo "summary csv not found: $summary_csv" >&2
	exit 1
fi

echo
echo "Evaluating summary: $summary_csv"
if ! awk -F, -v thr="$MAX_MEDIAN_DELTA_PERCENT" -v re="$CORE_BENCH_REGEX" '
BEGIN {
  fail=0
}
NR == 1 {
  next
}
$1 ~ re {
  med = $6
  gsub(/[%+]/, "", med)
  med += 0
  if (med > thr) {
    printf "FAIL %s median_delta=%+.2f%% exceeds +%.2f%%\n", $1, med, thr
    fail = 1
  } else {
    printf "PASS %s median_delta=%+.2f%% <= +%.2f%%\n", $1, med, thr
  }
}
END {
  if (fail) {
    exit 2
  }
}
' "$summary_csv"; then
	echo "Performance gate failed." >&2
	exit 2
fi

echo "Performance gate passed."
