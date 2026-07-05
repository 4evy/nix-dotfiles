#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs

require_arg_count 1 1 "$@"
bin_dir=$1
require_value 'bin dir' "$bin_dir"
require_safe_path "$bin_dir"
require_command id launchctl ln

app_dst=/Applications/RustDesk.app
settings_dir="$HOME/Library/Preferences/com.carriez.RustDesk"
settings_file="$settings_dir/RustDesk2.toml"
settings_source="$script_dir/rustdesk/RustDesk2.toml"
launch_agents_dir="$HOME/Library/LaunchAgents"
launch_agent="$launch_agents_dir/com.carriez.RustDesk.plist"
launch_agent_source="$script_dir/rustdesk/com.carriez.RustDesk.plist"
launch_label=com.carriez.RustDesk
launch_domain="gui/$(id -u)"

require_file "$settings_source"
require_file "$launch_agent_source"

if [[ -x "$app_dst/Contents/MacOS/RustDesk" ]]; then
	ensure_dir "$bin_dir"
	ln -sfnT "$app_dst/Contents/MacOS/RustDesk" "$bin_dir/rustdesk"
fi

install_file_if_changed "$settings_source" "$settings_file" 0644
install_file_if_changed "$launch_agent_source" "$launch_agent" 0644
launchctl bootout "$launch_domain" "$launch_agent" >/dev/null 2>&1 || true
launchctl bootstrap "$launch_domain" "$launch_agent"
launchctl enable "$launch_domain/$launch_label"
launchctl kickstart -k "$launch_domain/$launch_label" >/dev/null 2>&1 || true

cat >&2 <<EOF
rustdesk-macos: configured direct IP access in $settings_file
rustdesk-macos: enabled login startup via $launch_agent
rustdesk-macos: grant RustDesk Accessibility and Screen Recording in System Settings > Privacy & Security.
rustdesk-macos: grant Input Monitoring too if keyboard or mouse input does not work.
EOF
