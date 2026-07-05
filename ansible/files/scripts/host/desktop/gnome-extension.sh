#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib fs
source_host_lib gnome
source_host_lib host
source_host_lib hyper_window_tiling

require_arg_count 3 3 "$@"

attr=${1:?extension source is required}
uuid=${2:?GNOME extension UUID is required}
label=${3:?extension label is required}

if ! host_gnome_shell_available; then
	printf '%s\n' "$label: GNOME Shell is not available; skipping"
	exit 0
fi

require_command install

repo_dir=$DOTFILES_REPO_ROOT
case $attr in
	hyper-window-tiling-gnome)
		build_root=$(hyper_window_tiling_build "$repo_dir/dotfiles")
		source_dir="$build_root/gnome/$uuid"
		;;
	*)
		die "$label: unsupported source-built GNOME extension attr: $attr"
		;;
esac

host_data_home=$(run_host_user_bash "printf \"%s\n\" \"\${XDG_DATA_HOME:-\$HOME/.local/share}\"")
destination="$host_data_home/gnome-shell/extensions/$uuid"

require_file "$source_dir/metadata.json"
require_file "$source_dir/extension.js"
require_dir "$source_dir/schemas"
if host_has_command glib-compile-schemas; then
	run_host_user glib-compile-schemas "$source_dir/schemas"
fi

ensure_dir "${destination%/*}"
if [[ -e $destination ]]; then
	chmod -R u+rwX "$destination"
fi
replace_dir "$source_dir" "$destination"

run_host_user gnome-extensions enable "$uuid" >/dev/null 2>&1 || true
host_enable_gnome_extensions "$uuid"

printf '%s\n' "$label: installed GNOME extension $uuid"
