#!/bin/sh
# Exercises check-action-pins.sh against a temporary fixture repository so the
# acceptance and rejection behavior is verified without touching repository
# workflows.

set -eu

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
fixture_root=$(mktemp -d)
trap 'rm -rf "$fixture_root"' EXIT

git init -q "$fixture_root"
mkdir -p "$fixture_root/.github/workflows"

cat >"$fixture_root/.github/workflows/pinned.yml" <<'EOF'
name: Pinned
on: [push]
jobs:
  one:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: ./.github/actions/local-helper
EOF

cat >"$fixture_root/.github/workflows/loose.yml" <<'EOF'
name: Loose
on: [push]
jobs:
  one:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303
      - uses: actions/setup-node@main
      - uses: docker/build-push-action
EOF

git -C "$fixture_root" add .github

if output=$(cd "$fixture_root" && "$script_directory/check-action-pins.sh" 2>&1); then
	printf 'expected loose fixture references to fail\n' >&2
	exit 1
fi
printf '%s\n' "$output" | grep -q 'loose.yml:7: uses is not pinned to a full commit SHA: actions/checkout@v7.0.1'
printf '%s\n' "$output" | grep -q 'loose.yml:8: uses is not pinned to a full commit SHA: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303'
printf '%s\n' "$output" | grep -q 'loose.yml:9: uses is not pinned to a full commit SHA: actions/setup-node@main'
printf '%s\n' "$output" | grep -q 'loose.yml:10: uses is not pinned to a commit SHA: docker/build-push-action'

rm "$fixture_root/.github/workflows/loose.yml"
git -C "$fixture_root" add .github

(cd "$fixture_root" && "$script_directory/check-action-pins.sh")

printf 'action pin fixture checks passed\n'
