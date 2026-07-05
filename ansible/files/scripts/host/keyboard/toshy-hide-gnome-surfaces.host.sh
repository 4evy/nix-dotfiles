#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

set_no_display() {
	local desktop_file=${1:?desktop file is required}

	if [[ ! -f $desktop_file ]]; then
		return 0
	fi

	require_command cat grep sed
	if grep -q "^[[:space:]]*NoDisplay=" "$desktop_file"; then
		sed "s/^[[:space:]]*NoDisplay=.*/NoDisplay=true/" "$desktop_file" |
			write_stdin_if_changed "$desktop_file" 0644
	else
		{
			cat "$desktop_file"
			printf "\nNoDisplay=true\n"
		} |
			write_stdin_if_changed "$desktop_file" 0644
	fi
}

data_home=${XDG_DATA_HOME:-$HOME/.local/share}
config_home=${XDG_CONFIG_HOME:-$HOME/.config}
for desktop_file in \
	"$data_home/applications/Toshy_Tray.desktop" \
	"$data_home/applications/app.toshy.preferences.desktop" \
	"$config_home/autostart/Toshy_Tray.desktop" \
	"$config_home/autostart/app.toshy.preferences.desktop"; do
	set_no_display "$desktop_file"
done

if command -v systemctl >/dev/null 2>&1; then
	systemctl --user disable --now toshy-tray.service >/dev/null 2>&1 || true
fi
