#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib host

require_arg_count 0 0 "$@"
bindings_helper="$script_dir/emoji-bindings.py"
require_file "$bindings_helper"

if ! gsettings_available; then
	printf '%s\n' 'emoji-picker-shortcut: gsettings is not available; skipping'
	exit 0
fi

if ! gsettings_writable org.freedesktop.ibus.panel.emoji hotkey; then
	printf '%s\n' 'emoji-picker-shortcut: IBus emoji settings are not available; skipping'
	exit 0
fi

python_command=python3
for candidate in /usr/bin/python3 /bin/python3; do
	if [[ -x $candidate ]]; then
		python_command=$candidate
		break
	fi
done

binding_path=/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/emoji-picker/
emoji_picker_command=${HOME}/.local/bin/carmenta-activate
if [[ ! -x $emoji_picker_command ]]; then
	printf 'emoji-picker-shortcut: %s is not executable; skipping\n' "$emoji_picker_command"
	exit 0
fi

if gsettings_writable org.gnome.settings-daemon.plugins.media-keys custom-keybindings; then
	current=$(gsettings get org.gnome.settings-daemon.plugins.media-keys custom-keybindings)
	next=$("$python_command" "$bindings_helper" "$binding_path" "$current")

	gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "$next"
	gsettings set org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:"$binding_path" name 'Emoji Picker'
	gsettings set org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:"$binding_path" command "$emoji_picker_command"
	gsettings set org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:"$binding_path" binding '<Control>period'
fi

gsettings set org.freedesktop.ibus.panel.emoji hotkey "['<Super>period', '<Super>semicolon']"

service_file=${HOME}/.config/systemd/user/carmenta-prewarm.service
if command -v systemctl >/dev/null 2>&1 && [[ -f $service_file ]]; then
	systemctl --user daemon-reload
	systemctl --user enable carmenta-prewarm.service >/dev/null 2>&1 || true
fi

printf '%s\n' 'emoji-picker-shortcut: bound Ctrl+. to Carmenta'
