#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs
source_host_lib template

require_arg_count 2 2 "$@"
kanata_config=$1
kanata_pebble_config=$2
require_file "$kanata_config"
require_file "$kanata_pebble_config"
require_command awk chown codesign install launchctl

label=dev.4evy.kanata
pebble_label=dev.4evy.kanata-pebble
app=/Applications/Kanata.app
app_bin="$app/Contents/MacOS/kanata"
info_plist="$app/Contents/Info.plist"
kanata_bin=/opt/homebrew/bin/kanata

require_executable "$kanata_bin"

ensure_dir_mode 0755 "$app/Contents/MacOS"
install_file_if_changed "$kanata_bin" "$app_bin" 0755
render_template "$script_dir/templates/kanata-app-info.plist.in" "$info_plist" \
	BUNDLE_IDENTIFIER "$label"
chown -R root:wheel "$app"
codesign --force --sign - "$app"
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f "$app"

write_daemon() {
	local daemon_label=$1
	local config_path=$2
	local log_path=$3
	local plist="/Library/LaunchDaemons/$daemon_label.plist"

	launchctl bootout system "$plist" >/dev/null 2>&1 || true
	render_template "$script_dir/templates/kanata-daemon.plist.in" "$plist" \
		LABEL "$daemon_label" \
		APP_BIN "$app_bin" \
		CONFIG_PATH "$config_path" \
		LOG_PATH "$log_path"
	chown root:wheel "$plist"
	launchctl bootstrap system "$plist"
	launchctl enable "system/$daemon_label"
	launchctl kickstart -k "system/$daemon_label"
}

write_daemon "$label" "$kanata_config" /var/log/kanata.log
write_daemon "$pebble_label" "$kanata_pebble_config" /var/log/kanata-pebble.log
