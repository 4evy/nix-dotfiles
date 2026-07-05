#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs

require_arg_count 4 5 "$@"
root=$1
bin_dir=$2
installer_bin=$3
secrets_file=$4
flags=${5:-}
require_value 'cache root' "$root"
require_value 'bin dir' "$bin_dir"
require_value 'installer bin dir' "$installer_bin"
require_safe_path "$root"
require_safe_path "$bin_dir"
require_safe_path "$installer_bin"
require_file "$secrets_file"
require_command go sops

ensure_dir "$root"
ensure_dir "$bin_dir"
ensure_dir "$installer_bin"

app_dir=/Applications/Helium.app
require_dir "$app_dir"

GOBIN="$installer_bin" go install ./cmd/helium-browser
"$installer_bin/helium-browser" configure \
	--secrets "$secrets_file" \
	-- \
	macos \
	"$root" \
	"$app_dir" \
	"$bin_dir" \
	"$flags"
