#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BENCH_DIR="$ROOT_DIR/benchmark"
cd "$BENCH_DIR"

RUNS=${RUNS:-4}
BENCHTIME=${BENCHTIME:-500ms}
GOMAXPROCS_VALUE=${GOMAXPROCS_VALUE:-1}
OUTPUT_DIR=${OUTPUT_DIR:-results/observed_ab}
STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
OUT_PREFIX=${OUT_PREFIX:-pslog_observed_ab_${STAMP}}
BENCH_PREFIX="BenchmarkPSLogProductionObservedAB/pslog/production"

variants=(
	"json"
	"json+with"
	"json+keyvals"
	"jsoncolor"
	"jsoncolor+with"
	"jsoncolor+keyvals"
	"console"
	"consolecolor"
)

if [[ ! "$RUNS" =~ ^[1-9][0-9]*$ ]]; then
	echo "RUNS must be a positive integer, got: $RUNS" >&2
	exit 1
fi

mkdir -p "$OUTPUT_DIR"

echo "Running $RUNS snapshot(s) for complete pslog observed-writer A/B:"
echo "  benchmark: $BENCH_PREFIX"
echo "  benchtime: $BENCHTIME"
echo "  output:    $OUTPUT_DIR"
echo

raw_csv="$OUTPUT_DIR/${OUT_PREFIX}_raw.csv"
echo "variant,observed,run,ns_per_op,bytes_per_op,allocs_per_op" >"$raw_csv"

run_one() {
	local variant="$1"
	local observed="$2"
	local run_index="$3"
	local run_out="$4"
	local escaped_variant
	local regex
	local output
	local line
	local ns
	local bytes
	local allocs

	escaped_variant=${variant//+/\\+}
	regex="^${BENCH_PREFIX}/${escaped_variant}$"
	if [[ "$observed" == "yes" ]]; then
		regex="^${BENCH_PREFIX}/${escaped_variant}\\+observed$"
	fi

	output=$(
		env -u NO_COLOR GOMAXPROCS="$GOMAXPROCS_VALUE" \
			go test -run '^$' -bench "$regex" -benchmem -count=1 -benchtime "$BENCHTIME" ./...
	)
	printf '%s\n' "$output" >>"$run_out"
	line=$(printf '%s\n' "$output" | awk '/^BenchmarkPSLogProductionObservedAB\/pslog\/production\// {print; exit}')

	if [[ -z "$line" ]]; then
		echo "failed to parse benchmark row for variant=$variant observed=$observed run=$run_index" >&2
		exit 1
	fi

	ns=$(awk '{print $3}' <<<"$line")
	bytes=$(awk '{print $5}' <<<"$line")
	allocs=$(awk '{print $9}' <<<"$line")
	printf '%s,%s,%d,%s,%s,%s\n' "$variant" "$observed" "$run_index" "$ns" "$bytes" "$allocs" >>"$raw_csv"
}

for ((run = 1; run <= RUNS; run++)); do
	run_out="$OUTPUT_DIR/${OUT_PREFIX}_run${run}.txt"
	echo "[run $run/$RUNS] -> $run_out"
	: >"$run_out"

	for variant in "${variants[@]}"; do
		if ((run % 2 == 1)); then
			run_one "$variant" "no" "$run" "$run_out"
			run_one "$variant" "yes" "$run" "$run_out"
		else
			run_one "$variant" "yes" "$run" "$run_out"
			run_one "$variant" "no" "$run" "$run_out"
		fi
	done
done

avg_csv="$OUTPUT_DIR/${OUT_PREFIX}_avg.csv"
tmp_avg_body=$(mktemp)
tmp_delta_body=""
trap 'rm -f "$tmp_avg_body" "${tmp_delta_body:-}"' EXIT
awk -F, '
NR == 1 { next }
{
  key = $1 "|" $2
  ns[key] += $4
  bytes[key] += $5
  allocs[key] += $6
  count[key]++
}
END {
  for (key in count) {
    split(key, parts, "|")
    variant = parts[1]
    observed = parts[2]
    printf "%s,%s,%d,%.3f,%.3f,%.3f\n", variant, observed, count[key], ns[key]/count[key], bytes[key]/count[key], allocs[key]/count[key]
  }
}
' "$raw_csv" >"$tmp_avg_body"
{
	echo "variant,observed,runs,avg_ns_per_op,avg_bytes_per_op,avg_allocs_per_op"
	sort "$tmp_avg_body"
} >"$avg_csv"

delta_csv="$OUTPUT_DIR/${OUT_PREFIX}_delta.csv"
tmp_delta_body=$(mktemp)
awk -F, '
NR == 1 { next }
{
  variant = $1
  observed = $2
  runs = $3
  ns = $4 + 0
  bytes = $5 + 0
  allocs = $6 + 0
  seen[variant] = 1
  if (observed == "no") {
    base_runs[variant] = runs
    base_ns[variant] = ns
    base_bytes[variant] = bytes
    base_allocs[variant] = allocs
  } else if (observed == "yes") {
    obs_runs[variant] = runs
    obs_ns[variant] = ns
    obs_bytes[variant] = bytes
    obs_allocs[variant] = allocs
  }
}
END {
  for (variant in seen) {
    if (!(variant in base_ns) || !(variant in obs_ns)) {
      printf "%s,NA,NA,NA,NA,NA,NA,NA,NA,NA,NA\n", variant
      continue
    }
    d = obs_ns[variant] - base_ns[variant]
    p = (base_ns[variant] == 0) ? 0 : (d / base_ns[variant]) * 100
    printf "%s,%s,%s,%.3f,%.3f,%+.3f,%+.2f%%,%.3f,%.3f,%.3f,%.3f\n", variant, base_runs[variant], obs_runs[variant], base_ns[variant], obs_ns[variant], d, p, base_bytes[variant], obs_bytes[variant], base_allocs[variant], obs_allocs[variant]
  }
}
' "$avg_csv" >"$tmp_delta_body"
{
	echo "variant,baseline_runs,observed_runs,baseline_ns_per_op,observed_ns_per_op,delta_ns_per_op,delta_percent,baseline_bytes_per_op,observed_bytes_per_op,baseline_allocs_per_op,observed_allocs_per_op"
	sort "$tmp_delta_body"
} >"$delta_csv"

echo
echo "Averages: $avg_csv"
cat "$avg_csv"
echo
echo "Delta report: $delta_csv"
if command -v column >/dev/null 2>&1; then
	column -s, -t "$delta_csv"
else
	cat "$delta_csv"
fi
