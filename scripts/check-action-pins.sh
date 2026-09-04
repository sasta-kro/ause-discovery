#!/bin/sh
# Validates that tracked GitHub Actions workflow files pin every external
# "uses:" reference to a full 40-character commit SHA. Local composite actions
# referenced with a ./path prefix are allowed.

set -u

violations=$(git ls-files -- '.github/workflows' | while IFS= read -r file; do
	[ -f "$file" ] || continue
	awk '
		function report(message) {
			printf "%s:%d: %s\n", FILENAME, FNR, message
		}
		/^[[:space:]]*(-[[:space:]]+)?uses:/ {
			line = $0
			sub(/^[[:space:]]*(-[[:space:]]+)?uses:[[:space:]]*/, "", line)
			gsub(/"/, "", line)
			gsub(quote, "", line)
			sub(/[[:space:]]*#.*$/, "", line)
			sub(/[[:space:]]+$/, "", line)
			if (line == "") next
			if (substr(line, 1, 2) == "./") next
			at = index(line, "@")
			if (at == 0) {
				report("uses is not pinned to a commit SHA: " line)
				next
			}
			reference = substr(line, 1, at - 1)
			sha = substr(line, at + 1)
			if (length(sha) != 40 || sha !~ /^[0-9a-f]+$/ || reference !~ /^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/) {
				report("uses is not pinned to a full commit SHA: " line)
			}
		}
		BEGIN { quote = sprintf("%c", 39) }
	' "$file"
done)

if [ -n "$violations" ]; then
	printf '%s\n' "$violations" >&2
	printf 'pin every external action to a full 40-character commit SHA and add a nearby release comment\n' >&2
	exit 1
fi

printf 'all tracked workflow uses references are pinned to commit SHAs\n'
