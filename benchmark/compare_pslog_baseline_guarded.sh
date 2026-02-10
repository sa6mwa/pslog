#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BENCH_DIR="$ROOT_DIR/benchmark"
BASE_SCRIPT="$BENCH_DIR/compare_pslog_baseline.sh"

MAX_NORMALIZED_LOAD=${MAX_NORMALIZED_LOAD:-0.35}
MAX_TOP_CPU=${MAX_TOP_CPU:-75}
QUIET_RETRIES=${QUIET_RETRIES:-6}
QUIET_SLEEP=${QUIET_SLEEP:-5}
PIN_CORE=${PIN_CORE:-$(( $(getconf _NPROCESSORS_ONLN) - 1 ))}

if [[ ! "$QUIET_RETRIES" =~ ^[1-9][0-9]*$ ]]; then
	echo "QUIET_RETRIES must be a positive integer, got: $QUIET_RETRIES" >&2
	exit 1
fi

if [[ ! "$QUIET_SLEEP" =~ ^[1-9][0-9]*$ ]]; then
	echo "QUIET_SLEEP must be a positive integer (seconds), got: $QUIET_SLEEP" >&2
	exit 1
fi

if [[ ! "$PIN_CORE" =~ ^[0-9]+$ ]]; then
	echo "PIN_CORE must be a non-negative integer core index, got: $PIN_CORE" >&2
	exit 1
fi

collect_host_load() {
	local load1 nproc normalized top_cpu
	load1=$(awk '{print $1}' /proc/loadavg)
	nproc=$(getconf _NPROCESSORS_ONLN)
	normalized=$(awk -v l="$load1" -v n="$nproc" 'BEGIN { if (n <= 0) { print "1.0"; exit }; printf "%.4f", (l / n) }')
	top_cpu=$(ps -eo pcpu= --sort=-pcpu | awk 'NR==1 { print ($1 + 0) }')
	printf "%s %s %s %s\n" "$load1" "$nproc" "$normalized" "$top_cpu"
}

is_quiet_enough() {
	local normalized="$1"
	local top_cpu="$2"
	if ! awk -v current="$normalized" -v max="$MAX_NORMALIZED_LOAD" 'BEGIN { exit !(current <= max) }'; then
		return 1
	fi
	if ! awk -v current="$top_cpu" -v max="$MAX_TOP_CPU" 'BEGIN { exit !(current <= max) }'; then
		return 1
	fi
	return 0
}

quiet_ok=0
for ((attempt = 1; attempt <= QUIET_RETRIES; attempt++)); do
	read -r load1 nproc normalized top_cpu < <(collect_host_load)
	if is_quiet_enough "$normalized" "$top_cpu"; then
		quiet_ok=1
		echo "Host check passed at attempt $attempt/$QUIET_RETRIES: load1=$load1 normalized=$normalized top_cpu=${top_cpu}%"
		break
	fi
	echo "Host too noisy at attempt $attempt/$QUIET_RETRIES: load1=$load1 normalized=$normalized top_cpu=${top_cpu}% (limits normalized<=$MAX_NORMALIZED_LOAD, top_cpu<=$MAX_TOP_CPU)"
	if (( attempt < QUIET_RETRIES )); then
		sleep "$QUIET_SLEEP"
	fi
done

if (( quiet_ok == 0 )); then
	echo "Refusing benchmark run: host remained noisy after $QUIET_RETRIES checks." >&2
	exit 2
fi

if command -v taskset >/dev/null 2>&1; then
	echo "Running baseline comparison pinned to core $PIN_CORE"
	exec taskset -c "$PIN_CORE" "$BASE_SCRIPT" "$@"
fi

echo "taskset not found; running without CPU pinning" >&2
exec "$BASE_SCRIPT" "$@"
