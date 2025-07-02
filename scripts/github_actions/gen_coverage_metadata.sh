#!/bin/sh

set -ex

# This script generates coverage metadata for the coverage report.
# Output is used by SonarQube integration in Uber and not used by OS repo coverage tool itself.

# Example output:
#   commit-sha: 6953daa563e8e44512bc349c9608484cfd4ec4ff
#   timestamp: 2024-03-04T19:29:16Z

output_path="$1"

echo "commit-sha: $(git rev-parse HEAD)" > "$output_path"
echo "timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$output_path"

echo "Coverage metadata written to $output_path"
