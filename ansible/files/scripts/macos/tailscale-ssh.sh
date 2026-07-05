#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs
source_host_lib template

require_command awk chown launchctl rm

label=dev.4evy.tailscale-ssh
old_label=com.tailscale.tailscale-ssh
support_dir="/Library/Application Support/dotfiles"
helper="$support_dir/tailscale-ssh.sh"
plist="/Library/LaunchDaemons/$label.plist"
old_plist="/Library/LaunchDaemons/$old_label.plist"
tailscale_bin=/opt/homebrew/bin/tailscale

require_executable "$tailscale_bin"
ensure_dir "$support_dir"

render_template_mode 0755 "$script_dir/templates/tailscale-ssh-helper.sh.in" "$helper" \
	TAILSCALE_BIN "$tailscale_bin"

if [[ -f $old_plist ]]; then
	launchctl bootout system "$old_plist" >/dev/null 2>&1 || true
	rm -f "$old_plist"
fi
launchctl bootout system "$plist" >/dev/null 2>&1 || true

render_template "$script_dir/templates/tailscale-ssh.plist.in" "$plist" \
	LABEL "$label" \
	HELPER "$helper"
chown root:wheel "$plist" "$helper"

launchctl bootstrap system "$plist"
launchctl enable "system/$label"
launchctl kickstart -k "system/$label"
