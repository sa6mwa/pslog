#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BENCH_SUBDIR=benchmark

BASE_COMMIT=${BASE_COMMIT:-}
CANDIDATE_COMMIT=${CANDIDATE_COMMIT:-HEAD}
RUNS=${RUNS:-4}
BENCH_REGEX=${BENCH_REGEX:-^BenchmarkPSLogProduction$}
BENCHTIME=${BENCHTIME:-700ms}
GOMAXPROCS_VALUE=${GOMAXPROCS_VALUE:-1}
PIN_CORE=${PIN_CORE:-}
OUTPUT_DIR=${OUTPUT_DIR:-results/commit_ab}
STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
OUT_PREFIX=${OUT_PREFIX:-commit_ab_${STAMP}}

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

if [[ -n "$PIN_CORE" && ! "$PIN_CORE" =~ ^[0-9]+$ ]]; then
	echo "PIN_CORE must be a non-negative integer core index, got: $PIN_CORE" >&2
	exit 1
fi

if [[ -n "$PIN_CORE" ]] && ! command -v taskset >/dev/null 2>&1; then
	echo "PIN_CORE was set but taskset is unavailable on this host." >&2
	exit 1
fi

mkdir -p "$OUTPUT_DIR"

tmp_base=$(mktemp -d /tmp/pslog-base-commit.XXXXXX)
tmp_candidate=$(mktemp -d /tmp/pslog-candidate-commit.XXXXXX)

cleanup() {
	git -C "$ROOT_DIR" worktree remove --force "$tmp_base" >/dev/null 2>&1 || true
	git -C "$ROOT_DIR" worktree remove --force "$tmp_candidate" >/dev/null 2>&1 || true
	rm -rf "$tmp_base" "$tmp_candidate"
}
trap cleanup EXIT

git -C "$ROOT_DIR" worktree add --detach "$tmp_base" "$BASE_COMMIT" >/dev/null
git -C "$ROOT_DIR" worktree add --detach "$tmp_candidate" "$CANDIDATE_COMMIT" >/dev/null

base_sha=$(git -C "$tmp_base" rev-parse --short HEAD)
candidate_sha=$(git -C "$tmp_candidate" rev-parse --short HEAD)

echo "Commit AB comparison:"
echo "  base:      $base_sha ($BASE_COMMIT)"
echo "  candidate: $candidate_sha ($CANDIDATE_COMMIT)"
echo "  runs:      $RUNS"
echo "  benchmark: $BENCH_REGEX"
echo "  benchtime: $BENCHTIME"
if [[ -n "$PIN_CORE" ]]; then
	echo "  pin core:  $PIN_CORE"
else
	echo "  pin core:  disabled"
fi
echo "  output:    $OUTPUT_DIR"
echo

run_bench_once() {
	local worktree="$1"
	local out_file="$2"
	local cmd=(
		env -u NO_COLOR GOMAXPROCS="$GOMAXPROCS_VALUE"
		go test -run '^$' -bench "$BENCH_REGEX" -benchmem -count=1 -benchtime "$BENCHTIME" ./...
	)
	if [[ -n "$PIN_CORE" ]]; then
		cmd=(taskset -c "$PIN_CORE" "${cmd[@]}")
	fi
	(
		cd "$worktree/$BENCH_SUBDIR"
		"${cmd[@]}"
	) | tee "$out_file" >/dev/null
}

pairs_csv="$OUTPUT_DIR/${OUT_PREFIX}_pairs.csv"
summary_csv="$OUTPUT_DIR/${OUT_PREFIX}_summary.csv"
tmp_rows=$(mktemp)
tmp_pairs_body=$(mktemp)
tmp_summary_body=$(mktemp)

for ((pair = 1; pair <= RUNS; pair++)); do
	base_out="$OUTPUT_DIR/${OUT_PREFIX}_pair${pair}_base.txt"
	candidate_out="$OUTPUT_DIR/${OUT_PREFIX}_pair${pair}_candidate.txt"

	if (( pair % 2 == 1 )); then
		echo "[pair $pair/$RUNS] order=base->candidate"
		run_bench_once "$tmp_base" "$base_out"
		run_bench_once "$tmp_candidate" "$candidate_out"
	else
		echo "[pair $pair/$RUNS] order=candidate->base"
		run_bench_once "$tmp_candidate" "$candidate_out"
		run_bench_once "$tmp_base" "$base_out"
	fi

	awk -v role="base" -v pair="$pair" '
	/^BenchmarkPSLogProduction\/pslog\/production\// {
	  printf "%s,%s,%s,%.3f,%.3f,%.3f\n", role, pair, $1, $3, $5, $9
	}' "$base_out" >>"$tmp_rows"

	awk -v role="candidate" -v pair="$pair" '
	/^BenchmarkPSLogProduction\/pslog\/production\// {
	  printf "%s,%s,%s,%.3f,%.3f,%.3f\n", role, pair, $1, $3, $5, $9
	}' "$candidate_out" >>"$tmp_rows"
done

if [[ ! -s "$tmp_rows" ]]; then
	echo "No benchmark rows were parsed from run outputs." >&2
	exit 1
fi

awk -F, '
{
  role=$1; pair=$2; bench=$3; ns=$4+0; bytes=$5+0; allocs=$6+0
  k=pair SUBSEP bench
  seen[k]=1
  benches[bench]=1
  if (role == "base") {
    base_ns[k]=ns; base_bytes[k]=bytes; base_allocs[k]=allocs
  } else {
    cand_ns[k]=ns; cand_bytes[k]=bytes; cand_allocs[k]=allocs
  }
}
END {
  for (k in seen) {
    if (!(k in base_ns) || !(k in cand_ns)) {
      continue
    }
    split(k, parts, SUBSEP)
    pair=parts[1]
    bench=parts[2]
    b=base_ns[k]
    c=cand_ns[k]
    d=c-b
    p=(b==0)?0:((d/b)*100)
    printf "%s,%s,%.3f,%.3f,%+.3f,%+.2f%%,%.3f,%.3f,%.3f,%.3f\n", pair, bench, b, c, d, p, base_bytes[k], cand_bytes[k], base_allocs[k], cand_allocs[k]
  }
}
' "$tmp_rows" | sort -t, -k2,2 -k1,1n >"$tmp_pairs_body"

{
	echo "pair,benchmark,base_ns_per_op,candidate_ns_per_op,delta_ns_per_op,delta_percent,base_bytes_per_op,candidate_bytes_per_op,base_allocs_per_op,candidate_allocs_per_op"
	cat "$tmp_pairs_body"
} >"$pairs_csv"

awk -F, '
{
  bench=$2
  b=$3+0
  c=$4+0
  d=$5+0
  p=$6
  gsub(/%/, "", p)
  n[bench]++
  base_sum[bench]+=b
  cand_sum[bench]+=c
  delta_sum[bench]+=d
  delta_pct_vals[bench, n[bench]]=(p+0)
}
END {
  for (bench in n) {
    m=n[bench]
    delete vals
    for (i=1; i<=m; i++) {
      vals[i]=delta_pct_vals[bench, i]
    }
    asort(vals)
    if (m % 2 == 1) {
      med=vals[(m+1)/2]
    } else {
      med=(vals[m/2] + vals[m/2 + 1]) / 2
    }
    mean=delta_sum[bench]/m
    mean_pct=(base_sum[bench] == 0) ? 0 : ((cand_sum[bench]-base_sum[bench]) / base_sum[bench]) * 100
    printf "%s,%d,%.3f,%.3f,%+.3f,%+.2f%%,%+.2f%%\n", bench, m, base_sum[bench]/m, cand_sum[bench]/m, mean, med, mean_pct
  }
}
' "$tmp_pairs_body" | sort -t, -k1,1 >"$tmp_summary_body"

{
	echo "benchmark,pairs,avg_base_ns_per_op,avg_candidate_ns_per_op,avg_delta_ns_per_op,median_delta_percent,mean_delta_percent"
	cat "$tmp_summary_body"
} >"$summary_csv"

echo
echo "Pairwise report: $pairs_csv"
if command -v column >/dev/null 2>&1; then
	column -s, -t "$pairs_csv"
else
	cat "$pairs_csv"
fi

echo
echo "Summary report: $summary_csv"
if command -v column >/dev/null 2>&1; then
	column -s, -t "$summary_csv"
else
	cat "$summary_csv"
fi

rm -f "$tmp_rows" "$tmp_pairs_body" "$tmp_summary_body"
