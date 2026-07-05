#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib fs
source_host_lib host

require_arg_count 0 0 "$@"

run_host_bash 'if command -v fc-cache >/dev/null 2>&1; then fc-cache -f; fi'

if command -v fc-cache >/dev/null 2>&1; then
	fc-cache -f || true
fi

fontconfig_cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/fontconfig"
if [[ -d $fontconfig_cache_dir ]]; then
	require_command find
	find "$fontconfig_cache_dir" -ignore_readdir_race -maxdepth 1 \( -type f -o -type l \) -name '*cache-11' -delete
fi

flatpak_fontconfig_root="${HOME}/.var/app"
if [[ -d $flatpak_fontconfig_root ]]; then
	require_command find install
	user_fontconfig_conf_dir="${XDG_CONFIG_HOME:-$HOME/.config}/fontconfig/conf.d"
	if [[ -d $user_fontconfig_conf_dir ]]; then
		while IFS= read -r -d '' app_dir; do
			app_fontconfig_conf_dir="$app_dir/config/fontconfig/conf.d"
			ensure_dir_mode 0755 "$app_fontconfig_conf_dir"
			find "$user_fontconfig_conf_dir" -maxdepth 1 -type f -name '*.conf' \
				-exec install -C -m 0644 -t "$app_fontconfig_conf_dir" -- {} +
		done < <(find "$flatpak_fontconfig_root" -mindepth 1 -maxdepth 1 -type d -print0)
	fi

	find "$flatpak_fontconfig_root" -ignore_readdir_race -path '*/cache/fontconfig/*cache-*' -type f -delete
fi
