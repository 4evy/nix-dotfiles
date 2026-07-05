#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib host

require_arg_count 0 0 "$@"

bash "$script_dir/gnome-extension.sh" \
	hyper-window-tiling-gnome \
	hyper-window-tiling@4evy.local \
	hyper-window-tiling

host_data_home=$(run_host_user_bash "printf \"%s\n\" \"\${XDG_DATA_HOME:-\$HOME/.local/share}\"")
schema_dir="$host_data_home/gnome-shell/extensions/hyper-window-tiling@4evy.local/schemas"
if [[ ! -d $schema_dir ]]; then
	exit 0
fi

run_host_user_bash_file "$script_dir/gnome-keys.host.sh" "$schema_dir"
