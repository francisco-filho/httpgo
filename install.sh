#!/usr/bin/env bash
# HTTPGo installer. Compatible with Bash 3.2; never reads from stdin.
set -euo pipefail

fail() { printf 'httpgo install: %s\n' "$*" >&2; exit 1; }
valid_tag() {
    local number='(0|[1-9][0-9]*)'
    local identifier='(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)'
    local pattern="^v${number}\.${number}\.${number}(-${identifier}(\.${identifier})*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$"
    [[ $1 =~ $pattern ]]
}
tag=''
bin_dir=${HOME:?HOME is required}/.local/bin
while [[ $# -gt 0 ]]; do
    case $1 in
        --version|--bin-dir)
            [[ $# -ge 2 && -n $2 && $2 != --* ]] || fail "missing value for $1"
            case $1 in
                --version) tag=$2; valid_tag "$tag" || fail "invalid version tag: $tag" ;;
                --bin-dir) bin_dir=$2 ;;
            esac
            shift 2
            ;;
        --help)
            printf 'Usage: bash install.sh [--version <vSemVer>] [--bin-dir <directory>]\n'
            exit 0
            ;;
        *) fail "unknown argument: $1 (see --help)" ;;
    esac
done
for tool in curl uname tar mktemp mkdir cp chmod mv rm; do
    command -v "$tool" >/dev/null 2>&1 || fail "required tool not found: $tool"
done
if command -v sha256sum >/dev/null 2>&1; then
    hash() { sha256sum "$1"; }
elif command -v shasum >/dev/null 2>&1; then
    hash() { shasum -a 256 "$1"; }
else
    fail 'SHA-256 verifier required: install sha256sum or shasum'
fi
system=$(uname -s) || fail 'cannot detect operating system'
case $system in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail "unsupported operating system: $system" ;;
esac
machine=$(uname -m) || fail 'cannot detect architecture'
case $machine in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) fail "unsupported architecture: $machine" ;;
esac
base=https://github.com/francisco-filho/httpgo/releases
# -q must be first: user curlrc must not disable TLS verification or add outputs.
fetch() { curl -q --fail --silent --show-error --location --proto '=https' --proto-redir '=https' "$@"; }
if [[ -z $tag ]]; then
    resolved=$(fetch --output /dev/null --write-out '%{url_effective}' "$base/latest") || fail 'cannot resolve latest stable release over HTTPS'
    case $resolved in
        "$base/tag/"*) tag=${resolved#"$base/tag/"} ;;
        *) fail 'latest did not resolve to an official release tag' ;;
    esac
    valid_tag "$tag" || fail 'latest resolved to an invalid version tag'
    case ${tag%%+*} in *-*) fail 'latest resolved to a prerelease, refusing installation' ;; esac
fi
asset=httpgo_${os}_${arch}.tar.gz
work=''
staged=''
cleanup() {
    [[ -z $staged ]] || rm -f "$staged"
    [[ -z $work ]] || rm -rf "$work"
    return 0
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
work=$(mktemp -d) || fail 'cannot create temporary download directory'
fetch --output "$work/$asset" "$base/download/$tag/$asset" || fail "cannot download $asset for $tag"
fetch --output "$work/checksums.txt" "$base/download/$tag/checksums.txt" || fail "cannot download checksums for $tag"
expected=''
while read -r digest filename extra || [[ -n ${digest:-} ]]; do
    if [[ ${filename:-} == "$asset" ]]; then
        [[ -z $expected && -z ${extra:-} && $digest =~ ^[0-9a-f]{64}$ ]] || fail "invalid or duplicate checksum for $asset"
        expected=$digest
    fi
done < "$work/checksums.txt"
[[ -n $expected ]] || fail "missing checksum for $asset"
actual=$(hash "$work/$asset") || fail "cannot compute SHA-256 for $asset"
[[ ${actual%% *} == "$expected" ]] || fail "SHA-256 mismatch for $asset"
# Never extract archive paths to disk: require exactly one regular member and
# stream only its bytes into a file we created. Reject links and extra members.
members=$(tar -tzf "$work/$asset") || fail 'invalid release archive'
[[ $members == httpgo ]] || fail 'unsafe archive: expected only httpgo'
details=$(tar -tvzf "$work/$asset") || fail 'cannot inspect release archive'
[[ $details == -* ]] || fail 'unsafe archive: httpgo must be a regular file'
tar -xOzf "$work/$asset" httpgo > "$work/httpgo" || fail 'cannot extract httpgo'
[[ -s $work/httpgo ]] || fail 'release executable is empty'
case $bin_dir in /*) ;; *) bin_dir=$PWD/$bin_dir ;; esac
mkdir -p "$bin_dir" || fail "cannot create bin directory: $bin_dir"
bin_dir=$(cd "$bin_dir" && pwd -P) || fail 'cannot resolve bin directory'
destination=$bin_dir/httpgo
[[ ! -d $destination ]] || fail "destination is a directory: $destination"
staged=$(mktemp "$bin_dir/.httpgo.XXXXXX") || fail "cannot prepare file in $bin_dir"
cp "$work/httpgo" "$staged" || fail 'cannot write staged executable'
chmod 755 "$staged" || fail 'cannot make staged executable runnable'
# Same-directory rename preserves the old executable on any preparation error.
mv -f "$staged" "$destination" || fail "cannot replace $destination"
staged=''
printf 'Installed httpgo %s at %s\n' "$tag" "$destination"
case :${PATH:-}: in
    *:"$bin_dir":*) ;;
    *) printf 'Add %s to your PATH to run httpgo by name\n' "$bin_dir" ;;
esac
