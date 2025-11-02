#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR/benchmark"

if ! command -v perf >/dev/null 2>&1; then
  echo "perf is required but was not found in PATH" >&2
  exit 1
fi

COUNT=${COUNT:-3}
BENCH=${BENCH:-BenchmarkPSLogProduction/pslog/production/json}
BENCHTIME=${BENCHTIME:-3s}
EVENTS=${PERF_EVENTS:-cycles,instructions}
CPU_CORE=${CPU_CORE:-}
GOMAXPROCS=${GOMAXPROCS:-1}
OUT_DIR=${OUT_DIR:-$ROOT_DIR/benchmark/results}
OUT_FILE=${OUT_FILE:-$OUT_DIR/pslog_json_perf.csv}

if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN=python3
elif command -v python >/dev/null 2>&1; then
  PYTHON_BIN=python
else
  echo "python or python3 is required" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

CMD=(go test -bench "$BENCH" -run ^$ -count=1 -benchtime "$BENCHTIME")

echo "Running pslog benchmark with perf counters"
echo "  COUNT=$COUNT"
echo "  BENCH=$BENCH"
echo "  BENCHTIME=$BENCHTIME"
echo "  PERF_EVENTS=$EVENTS"
echo "  CPU_CORE=${CPU_CORE:-auto}"
echo "  GOMAXPROCS=$GOMAXPROCS"
echo "  OUT_FILE=$OUT_FILE"
echo

export GOMAXPROCS

TASKSET_CMD=()
if [[ -n "$CPU_CORE" ]]; then
  TASKSET_CMD=(taskset -c "$CPU_CORE")
fi

if [[ ! -f "$OUT_FILE" ]]; then
  echo "timestamp,iteration,bench_iterations,ns_per_op,bytes_per_op,cycles,instructions,cycles_per_op,instructions_per_op,implied_ghz" > "$OUT_FILE"
fi

for (( run=1; run<=COUNT; run++ )); do
  timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  bench_tmp=$(mktemp)
  perf_tmp=$(mktemp)

  echo "[run $run/$COUNT] executing benchmark..."

  set +e
  "${TASKSET_CMD[@]}" perf stat -x, -e "$EVENTS" --log-fd 3 -- "${CMD[@]}" \
    1> >(tee "$bench_tmp") 2>"/dev/stderr" 3>"$perf_tmp"
  status=$?
  set -e

  if [[ $status -ne 0 ]]; then
    echo "run $run failed" >&2
    rm -f "$bench_tmp" "$perf_tmp"
    exit $status
  fi

  bench_line=$(grep -m1 "^$BENCH" "$bench_tmp" || true)
  if [[ -z "$bench_line" ]]; then
    echo "failed to parse benchmark output" >&2
    rm -f "$bench_tmp" "$perf_tmp"
    exit 1
  fi

  bench_iterations=$(echo "$bench_line" | awk '{print $2}')
  ns_per_op=$(echo "$bench_line" | awk '{print $3}')
  bytes_per_op=$(echo "$bench_line" | awk '{print $5}')

  cycles=$(awk -F',' '$3 ~ /cycles/ && $1 ~ /^[0-9]/ {print $1}' "$perf_tmp" | head -n1)
  instructions=$(awk -F',' '$3 ~ /instructions/ && $1 ~ /^[0-9]/ {print $1}' "$perf_tmp" | head -n1)

  if [[ -z "$cycles" || -z "$instructions" ]]; then
    echo "failed to parse perf output" >&2
    rm -f "$bench_tmp" "$perf_tmp"
    exit 1
  fi

  read -r cycles_per_op instructions_per_op implied_ghz <<<"$("$PYTHON_BIN" - "$bench_iterations" "$ns_per_op" "$cycles" "$instructions" <<'PY'
import math, sys
bench_iter = float(sys.argv[1])
ns_per_op = float(sys.argv[2])
cycles = float(sys.argv[3])
instructions = float(sys.argv[4])
cycles_per_op = cycles / bench_iter if bench_iter else float('nan')
instructions_per_op = instructions / bench_iter if bench_iter else float('nan')
implied_ghz = cycles_per_op / ns_per_op if ns_per_op else float('nan')
print(f"{cycles_per_op} {instructions_per_op} {implied_ghz}")
PY
)"

  printf 'cycles/op=%s instructions/op=%s impliedGHz=%s\n' "$cycles_per_op" "$instructions_per_op" "$implied_ghz"

  printf '%s,%d,%s,%s,%s,%s,%s,%s,%s,%s\n' \
    "$timestamp" "$run" "$bench_iterations" "$ns_per_op" "$bytes_per_op" "$cycles" "$instructions" \
    "$cycles_per_op" "$instructions_per_op" "$implied_ghz" >> "$OUT_FILE"

  rm -f "$bench_tmp" "$perf_tmp"
done

echo "Results appended to $OUT_FILE"
