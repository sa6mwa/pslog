#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BENCH_DIR="$ROOT_DIR/benchmark"
cd "$BENCH_DIR"

RUNS=${RUNS:-4}
BENCHTIME=${BENCHTIME:-700ms}
GOMAXPROCS_VALUE=${GOMAXPROCS_VALUE:-1}
OUTPUT_DIR=${OUTPUT_DIR:-results/observed_json_compare}
STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
OUT_PREFIX=${OUT_PREFIX:-json_observed_${STAMP}}
JSON_BENCH='^BenchmarkPSLogJSONObservedWriter/pslog/production/json$'
OBSERVED_BENCH='^BenchmarkPSLogJSONObservedWriter/pslog/production/json\+observed$'

if [[ ! "$RUNS" =~ ^[1-9][0-9]*$ ]]; then
	echo "RUNS must be a positive integer, got: $RUNS" >&2
	exit 1
fi

mkdir -p "$OUTPUT_DIR"

echo "Running $RUNS snapshot(s) for JSON observed-writer comparison:"
echo "  benchmark: BenchmarkPSLogJSONObservedWriter"
echo "  benchtime: $BENCHTIME"
echo "  output:    $OUTPUT_DIR"
echo

raw_csv="$OUTPUT_DIR/${OUT_PREFIX}_raw.csv"
echo "variant,run,ns_per_op,bytes_per_op,allocs_per_op" >"$raw_csv"

run_variant() {
	local variant="$1"
	local regex="$2"
	local run_index="$3"
	local run_out="$4"
	local output
	local line
	local ns
	local bytes
	local allocs

	output=$(
		env -u NO_COLOR GOMAXPROCS="$GOMAXPROCS_VALUE" \
			go test -run '^$' -bench "$regex" -benchmem -count=1 -benchtime "$BENCHTIME" ./...
	)
	printf '%s\n' "$output" >>"$run_out"
	line=$(printf '%s\n' "$output" | awk '/^BenchmarkPSLogJSONObservedWriter\// {print; exit}')

	if [[ -z "$line" ]]; then
		echo "failed to parse benchmark row for $variant (run $run_index)" >&2
		exit 1
	fi

	ns=$(awk '{print $3}' <<<"$line")
	bytes=$(awk '{print $5}' <<<"$line")
	allocs=$(awk '{print $9}' <<<"$line")
	printf '%s,%d,%s,%s,%s\n' "$variant" "$run_index" "$ns" "$bytes" "$allocs" >>"$raw_csv"
}

for ((i = 1; i <= RUNS; i++)); do
	run_out="$OUTPUT_DIR/${OUT_PREFIX}_run${i}.txt"
	echo "[run $i/$RUNS] -> $run_out"

	: >"$run_out"

	if ((i % 2 == 1)); then
		run_variant "json" "$JSON_BENCH" "$i" "$run_out"
		run_variant "json+observed" "$OBSERVED_BENCH" "$i" "$run_out"
	else
		run_variant "json+observed" "$OBSERVED_BENCH" "$i" "$run_out"
		run_variant "json" "$JSON_BENCH" "$i" "$run_out"
	fi
done

avg_csv="$OUTPUT_DIR/${OUT_PREFIX}_avg.csv"
awk -F, '
NR == 1 { next }
{
  ns[$1] += $3
  bytes[$1] += $4
  alloc[$1] += $5
  count[$1]++
}
END {
  print "variant,runs,avg_ns_per_op,avg_bytes_per_op,avg_allocs_per_op"
  if (count["json"] > 0) {
    printf "json,%d,%.3f,%.3f,%.3f\n", count["json"], ns["json"]/count["json"], bytes["json"]/count["json"], alloc["json"]/count["json"]
  }
  if (count["json+observed"] > 0) {
    printf "json+observed,%d,%.3f,%.3f,%.3f\n", count["json+observed"], ns["json+observed"]/count["json+observed"], bytes["json+observed"]/count["json+observed"], alloc["json+observed"]/count["json+observed"]
  }
}
' "$raw_csv" >"$avg_csv"

json_avg=$(awk -F, '$1=="json"{print $3}' "$avg_csv")
obs_avg=$(awk -F, '$1=="json+observed"{print $3}' "$avg_csv")
json_runs=$(awk -F, '$1=="json"{print $2}' "$avg_csv")
obs_runs=$(awk -F, '$1=="json+observed"{print $2}' "$avg_csv")

if [[ -z "$json_avg" || -z "$obs_avg" || -z "$json_runs" || -z "$obs_runs" ]]; then
	echo "failed to parse benchmark averages from $avg_csv" >&2
	exit 1
fi

delta_csv="$OUTPUT_DIR/${OUT_PREFIX}_delta.csv"
awk -v j="$json_avg" -v o="$obs_avg" -v jr="$json_runs" -v oruns="$obs_runs" '
BEGIN {
  d = o - j
  p = (j == 0) ? 0 : (d / j) * 100
  print "variant,runs,avg_ns_per_op,delta_ns_per_op,delta_percent"
  printf "json,%s,%.3f,%+.3f,%+.2f%%\n", jr, j, 0, 0
  printf "json+observed,%s,%.3f,%+.3f,%+.2f%%\n", oruns, o, d, p
}
' >"$delta_csv"

echo
echo "Averages: $avg_csv"
cat "$avg_csv"
echo
echo "Delta (json+observed vs json): $delta_csv"
cat "$delta_csv"
