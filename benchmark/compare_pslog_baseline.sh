#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BENCH_DIR="$ROOT_DIR/benchmark"
cd "$BENCH_DIR"

BASELINE_CSV=${BASELINE_CSV:-results/baseline_chunk1_pre/average_pslog.csv}
RUNS=${RUNS:-3}
BENCH_REGEX=${BENCH_REGEX:-^BenchmarkPSLogProduction$}
BENCHTIME=${BENCHTIME:-500ms}
GOMAXPROCS_VALUE=${GOMAXPROCS_VALUE:-1}
OUTPUT_DIR=${OUTPUT_DIR:-results/compare}
STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
OUT_PREFIX=${OUT_PREFIX:-pslog_vs_baseline_${STAMP}}

if [[ ! -f "$BASELINE_CSV" ]]; then
  echo "baseline csv not found: $BASELINE_CSV" >&2
  exit 1
fi

if [[ ! "$RUNS" =~ ^[1-9][0-9]*$ ]]; then
  echo "RUNS must be a positive integer, got: $RUNS" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

echo "Running $RUNS benchmark snapshot(s) against baseline:"
echo "  baseline: $BASELINE_CSV"
echo "  bench:    $BENCH_REGEX"
echo "  benchtime:$BENCHTIME"
echo "  output:   $OUTPUT_DIR"
echo

for ((i = 1; i <= RUNS; i++)); do
  run_out="$OUTPUT_DIR/${OUT_PREFIX}_run${i}.txt"
  echo "[run $i/$RUNS] -> $run_out"
  env -u NO_COLOR GOMAXPROCS="$GOMAXPROCS_VALUE" \
    go test -run '^$' -bench "$BENCH_REGEX" -benchmem -count=1 -benchtime "$BENCHTIME" ./... \
    | tee "$run_out" >/dev/null
done

tmp_body=$(mktemp)
trap 'rm -f "$tmp_body"' EXIT

awk '
/^BenchmarkPSLogProduction\/pslog\/production\// {
  name=$1
  ns[name]+=$3
  bytes[name]+=$5
  bop[name]+=$7
  alloc[name]+=$9
  count[name]++
}
END {
  for (name in count) {
    printf "%s,%d,%.3f,%.3f,%.3f,%.3f\n", name, count[name], ns[name]/count[name], bytes[name]/count[name], bop[name]/count[name], alloc[name]/count[name]
  }
}
' "$OUTPUT_DIR/${OUT_PREFIX}"_run*.txt >"$tmp_body"

if [[ ! -s "$tmp_body" ]]; then
  echo "no pslog production benchmark rows found in run output" >&2
  exit 1
fi

current_avg_csv="$OUTPUT_DIR/${OUT_PREFIX}_current_avg.csv"
{
  echo "benchmark,runs,avg_ns_per_op,avg_bytes_per_op,avg_B_per_op,avg_allocs_per_op"
  sort "$tmp_body"
} >"$current_avg_csv"

delta_csv="$OUTPUT_DIR/${OUT_PREFIX}_delta.csv"
awk -F, '
FNR == NR {
  if (FNR > 1) {
    base[$1] = $3
  }
  next
}
FNR > 1 {
  name = $1
  runs = $2
  cur = $3 + 0
  curBytes = $4 + 0
  curAllocs = $6 + 0
  seen[name] = 1
  if (name in base) {
    b = base[name] + 0
    d = cur - b
    p = (b == 0) ? 0 : (d / b) * 100
    printf "%s,%s,%.3f,%.3f,%+.3f,%+.2f%%,%.3f,%.3f\n", name, runs, b, cur, d, p, curBytes, curAllocs
  } else {
    printf "%s,%s,NA,%.3f,NA,NA,%.3f,%.3f\n", name, runs, cur, curBytes, curAllocs
  }
}
END {
  for (name in base) {
    if (!(name in seen)) {
      printf "%s,NA,%.3f,NA,NA,NA,NA,NA\n", name, base[name] + 0
    }
  }
}
' "$BASELINE_CSV" "$current_avg_csv" | {
  echo "benchmark,runs,baseline_ns_per_op,current_ns_per_op,delta_ns_per_op,delta_percent,current_bytes_per_op,current_allocs_per_op"
  sort
} >"$delta_csv"

echo
echo "Delta report: $delta_csv"
if command -v column >/dev/null 2>&1; then
  column -s, -t "$delta_csv"
else
  cat "$delta_csv"
fi

