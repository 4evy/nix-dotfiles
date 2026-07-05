#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=${DOTFILES_HOST_SCRIPT_DIR:-}
if [[ -z $script_dir ]]; then
	script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
fi

data_home=${XDG_DATA_HOME:-$HOME/.local/share}
config_home=${XDG_CONFIG_HOME:-$HOME/.config}
applications_dir="$data_home/applications"
autostart_file="$config_home/autostart/rustdesk.desktop"
desktop_file="$applications_dir/rustdesk.desktop"
source_desktop_file="$script_dir/rustdesk.desktop"

require_command cat
require_file "$source_desktop_file"
ensure_dir "$applications_dir"
install_file_if_changed "$source_desktop_file" "$desktop_file" 0644

if [[ -f $autostart_file ]]; then
	{
		cat "$desktop_file"
		printf "%s\n" "X-GNOME-Autostart-enabled=true"
	} |
		write_stdin_if_changed "$autostart_file" 0644
fi

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "$applications_dir" >/dev/null 2>&1 || true
fi
