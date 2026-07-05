#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib cache
source_host_lib gnome
source_host_lib http
source_host_lib json

require_arg_count 0 0 "$@"
require_command curl jq

if ! host_gnome_shell_available; then
	printf '%s\n' 'toshy-gnome-window-context: GNOME Shell is not available; skipping'
	exit 0
fi

shell_version=$(host_gnome_shell_major_version)
require_non_empty "$shell_version" 'toshy-gnome-window-context: failed to detect host GNOME Shell version'

ego_origin=https://extensions.gnome.org
cache_dir=$(dotfiles_cache_dir gnome-extensions)
ensure_dir "$cache_dir"

declare -A encoded_uuids=(
	["focused-window-dbus@flexagoon.com"]=focused-window-dbus%40flexagoon.com
	["window-calls-extended@hseliger.eu"]=window-calls-extended%40hseliger.eu
	["xremap@k0kubun.com"]=xremap%40k0kubun.com
	["appindicatorsupport@rgcjonas.gmail.com"]=appindicatorsupport%40rgcjonas.gmail.com
)

declare -A labels=(
	["focused-window-dbus@flexagoon.com"]='Focused Window D-Bus'
	["window-calls-extended@hseliger.eu"]='Window Calls Extended'
	["xremap@k0kubun.com"]='Xremap'
	["appindicatorsupport@rgcjonas.gmail.com"]='AppIndicator and KStatusNotifierItem Support'
)

installed=0
installed_window_context=0
for uuid in \
	focused-window-dbus@flexagoon.com \
	window-calls-extended@hseliger.eu \
	xremap@k0kubun.com \
	appindicatorsupport@rgcjonas.gmail.com; do
	info=$(
		curl_stdout \
			"$ego_origin/extension-info/?uuid=${encoded_uuids[$uuid]}&shell_version=$shell_version" \
			2>/dev/null || true
	)
	if [[ -z $info ]]; then
		printf '%s\n' "toshy-gnome-window-context: ${labels[$uuid]} has no GNOME Shell $shell_version metadata; skipping"
		continue
	fi
	if ! download_url=$(jq_read_text '.download_url // empty' "$info") ||
		! version=$(jq_read_text '.version // empty' "$info"); then
		printf '%s\n' "toshy-gnome-window-context: ${labels[$uuid]} has no compatible GNOME Shell $shell_version build; skipping"
		continue
	fi

	archive="$cache_dir/$uuid-$version.shell-extension.zip"
	curl_download "$ego_origin$download_url" "$archive"

	installed_uuid=$(run_host_user gnome-extensions install --force --print-uuid "$archive")
	if [[ $installed_uuid != "$uuid" ]]; then
		die "toshy-gnome-window-context: installed UUID $installed_uuid did not match $uuid"
	fi
	run_host_user gnome-extensions enable "$uuid" >/dev/null 2>&1 || true
	host_enable_gnome_extensions "$uuid"
	installed=$((installed + 1))
	if [[ $uuid != appindicatorsupport@rgcjonas.gmail.com ]]; then
		installed_window_context=$((installed_window_context + 1))
	fi
	printf '%s\n' "toshy-gnome-window-context: installed ${labels[$uuid]} for GNOME Shell $shell_version"
done

if ((installed_window_context == 0)); then
	die "toshy-gnome-window-context: no Toshy-compatible GNOME window context extension was available for GNOME Shell $shell_version"
fi

run_host_user_bash_file "$script_dir/overlay.host.sh"
