#!/usr/bin/env bash
# Build release assets; this does not prove native execution on other targets.
set -euo pipefail

fail() { printf 'package-release: %s\n' "$*" >&2; exit 1; }
classify_tag() {
    local number='(0|[1-9][0-9]*)'
    local identifier='(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)'
    local pattern="^v${number}\.${number}\.${number}(-${identifier}(\.${identifier})*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$"
    [[ $1 =~ $pattern ]] || fail "invalid SemVer tag: $1"
    case ${1%%+*} in
        *-*) printf 'prerelease\n' ;;
        *) printf 'stable\n' ;;
    esac
}

if [[ ${1:-} == --validate-tag && $# == 2 ]]; then
    classify_tag "$2"
    exit
fi
[[ $# == 2 ]] || fail 'usage: scripts/package-release.sh <vSemVer> <new-output-directory>'
tag=$1
classify_tag "$tag" >/dev/null
root=$(cd "$(dirname "$0")/.." && pwd)
case $2 in
    /*) output=$2 ;;
    *) output=$PWD/$2 ;;
esac
[[ ! -e $output ]] || fail "output already exists: $output"
commit=${RELEASE_COMMIT:-$(git -C "$root" rev-parse HEAD)}
built_at=${RELEASE_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
[[ $commit =~ ^[0-9a-f]{40}$ ]] || fail 'commit must be a full lowercase SHA'
[[ $built_at =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || fail 'date must be UTC RFC3339'
if command -v sha256sum >/dev/null 2>&1; then
    hash() { sha256sum "$1"; }
elif command -v shasum >/dev/null 2>&1; then
    hash() { shasum -a 256 "$1"; }
else
    fail 'sha256sum or shasum is required'
fi
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir "$work/assets"
cd "$root"
for os in linux darwin; do
    for arch in amd64 arm64; do
        target="$work/$os-$arch"
        mkdir "$target"
        CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
            -ldflags "-X main.version=$tag -X main.commit=$commit -X main.date=$built_at" \
            -o "$target/httpgo" ./cmd/httpgo
        tar -czf "$work/assets/httpgo_${os}_${arch}.tar.gz" -C "$target" httpgo
    done
done
if [[ -f $root/install.sh ]]; then
    cp "$root/install.sh" "$work/assets/install.sh"
fi
(cd "$work/assets"; for asset in httpgo_*.tar.gz; do hash "$asset"; done > checksums.txt)
# Publish the output directory only after all builds and hashes succeed.
mkdir -p "$(dirname "$output")"
mv "$work/assets" "$output"
printf 'prepared %s in %s\n' "$tag" "$output"
