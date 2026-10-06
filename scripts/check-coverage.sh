#!/usr/bin/env bash
set -euo pipefail
awk -v overall="${2:-70}" -v client="${3:-80}" -v plugin="${4:-80}" '
NR > 1 {
    statements["overall"] += $2
    covered["overall"] += ($3 > 0 ? $2 : 0)
    if (index($1, "/internal/vantageapi/")) {
        statements["client"] += $2
        covered["client"] += ($3 > 0 ? $2 : 0)
    }
    if (index($1, "/internal/plugin/")) {
        statements["plugin"] += $2
        covered["plugin"] += ($3 > 0 ? $2 : 0)
    }
}
END {
    thresholds["overall"] = overall
    thresholds["client"] = client
    thresholds["plugin"] = plugin
    for (scope in thresholds) {
        percentage = statements[scope] ? 100 * covered[scope] / statements[scope] : 0
        printf "%s coverage: %.1f%% (minimum %.1f%%)\n", scope, percentage, thresholds[scope]
        if (!statements[scope] || percentage < thresholds[scope]) failed = 1
    }
    exit failed
}' "${1:?usage: check-coverage.sh profile [overall client plugin]}"
