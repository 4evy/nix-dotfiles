#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs
source_host_lib http
source_host_lib json

# shellcheck source=ansible/files/scripts/apps/helium-lib.sh
source -p "$script_dir" helium-lib.sh

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
require_command go sops tar sed

ensure_dir "$root"
ensure_dir "$bin_dir"
ensure_dir "$installer_bin"

repository=imputnet/helium-linux
arch=$(helium_arch)
release_json=$(helium_latest_release "$repository")
version=$(helium_release_tag "$release_json")
asset_name="helium-${version}-${arch}_linux.tar.xz"
download_url=$(helium_asset_download_url "$release_json" "$asset_name")
digest=$(helium_asset_digest "$release_json" "$asset_name")
archive="$root/$asset_name"
extract_dir="$root/extract"
app_dir="$root/app"
version_file="$app_dir/.helium-version"

if [[ ! -f $version_file || $(<"$version_file") != "$version" ]]; then
	ensure_dir "$root"
	helium_ensure_downloaded "$archive" "$download_url" "$digest"

	printf '%s\n' 'helium-browser: extracting application archive' >&2
	fresh_dir "$extract_dir"
	remove_path "$app_dir"
	tar -xJf "$archive" -C "$extract_dir"

	payload=
	for entry in "$extract_dir"/*; do
		if [[ -d $entry ]]; then
			payload=$entry
			break
		fi
	done
	require_value 'Helium extracted application directory' "$payload"

	printf '%s\n' 'helium-browser: installing application payload' >&2
	replace_dir "$payload" "$app_dir"
	if [[ -f $app_dir/helium-wrapper ]]; then
		sed 's/^CHROME_VERSION_EXTRA=.*/CHROME_VERSION_EXTRA=ansible/' "$app_dir/helium-wrapper" |
			write_stdin_if_changed "$app_dir/helium-wrapper" 0755
	fi
	printf '%s\n' "$version" | write_stdin_if_changed "$version_file" 0644
	remove_path "$extract_dir"
else
	printf 'helium-browser: application %s is already installed\n' "$version" >&2
fi

CGO_ENABLED=0 GOBIN="$installer_bin" go install ./cmd/helium-browser
"$installer_bin/helium-browser" configure \
	--secrets "$secrets_file" \
	-- \
	linux \
	"$root" \
	"$app_dir" \
	"$bin_dir" \
	"$flags"
