#!/usr/bin/env bash
set -eo pipefail

usage() {
	cat <<'EOF'
Usage: check-too-fresh-deps.sh [options]

Fails (exit 1) if any non-exempt Go module dependency's pinned version was
published more recently than the allowed age, as a guard against merging
freshly-published (and potentially compromised) dependencies.

Options:
  --days N              Age threshold in days (default: 14)
  --input JSON          JSON input to check, in the stream format produced
                         by `go list -m -json all`. Defaults to running
                         that command live.
  --exempt-prefix LIST  Comma-separated module path prefixes exempted from
                         blocking (still reported, just not blocking).
                         Pass an empty string to disable exemptions.
                         (default: github.com/cadence-workflow/)
  --now EPOCH           Override current time as unix epoch seconds.
                         (for testing)
  -h, --help            Show this help.
EOF
}

days=14
input=""
exempt_prefixes="github.com/cadence-workflow/"
now=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	--days)
		days="$2"
		shift 2
		;;
	--input)
		input="$2"
		shift 2
		;;
	--exempt-prefix)
		exempt_prefixes="$2"
		shift 2
		;;
	--now)
		now="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown argument: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

if [[ -z "$input" ]]; then
	input="$(go list -m -json all)"
fi

if [[ -z "$now" ]]; then
	now="$(date -u +%s)"
fi

exempt_json="$(jq -cn --arg s "$exempt_prefixes" '$s | split(",") | map(select(length > 0))')"

flagged_json="$(jq -s \
	--argjson days "$days" \
	--argjson now "$now" \
	--argjson exempt "$exempt_json" \
	'
	def is_exempt($path): any($exempt[]; . as $p | $path | startswith($p));
	[.[] | select(.Time) |
		(($now - (.Time | fromdateiso8601)) / 86400 | floor) as $age |
		select($age < $days) |
		{path: .Path, version: .Version, time: .Time, age: $age, eligible_in: ($days - $age), exempt: is_exempt(.Path)}
	]
	' <<<"$input")"

count="$(jq 'length' <<<"$flagged_json")"
if [[ "$count" -eq 0 ]]; then
	exit 0
fi

print_block() {
	jq -r '.[] | "module: \(.path)\nversion: \(.version)\npublished: \(.time)\ndays_since_published: \(.age)\ndays_until_eligible: \(.eligible_in)\n"'
}

exempt_rows_json="$(jq '[.[] | select(.exempt == true)]' <<<"$flagged_json")"
if [[ "$(jq 'length' <<<"$exempt_rows_json")" -gt 0 ]]; then
	echo "Exempted dependencies published within ${days} days (not blocking):"
	echo
	print_block <<<"$exempt_rows_json"
fi

blocking_json="$(jq '[.[] | select(.exempt == false)]' <<<"$flagged_json")"
blocking_count="$(jq 'length' <<<"$blocking_json")"
if [[ "$blocking_count" -gt 0 ]]; then
	echo "Dependencies published within ${days} days (blocking):"
	echo
	print_block <<<"$blocking_json"
	exit 1
fi

exit 0
