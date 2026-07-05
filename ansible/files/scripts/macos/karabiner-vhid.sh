#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs
source_host_lib template

require_arg_count 0 0 "$@"
require_command chown curl install installer launchctl mktemp rm shasum

if ((EUID != 0)); then
	die 'karabiner-vhid: this script must run as root'
fi

version=6.2.0
sha256=9e8c46239f0748161241e42444857901224e5c82f5b58a1731df4c70bf0736a8
package=Karabiner-DriverKit-VirtualHIDDevice-$version.pkg
url=https://github.com/pqrs-org/Karabiner-DriverKit-VirtualHIDDevice/releases/download/v$version/$package

manager=/Applications/.Karabiner-VirtualHIDDevice-Manager.app/Contents/MacOS/Karabiner-VirtualHIDDevice-Manager
daemon="/Library/Application Support/org.pqrs/Karabiner-DriverKit-VirtualHIDDevice/Applications/Karabiner-VirtualHIDDevice-Daemon.app/Contents/MacOS/Karabiner-VirtualHIDDevice-Daemon"
plist=/Library/LaunchDaemons/org.pqrs.Karabiner-VirtualHIDDevice-Daemon.plist
label=org.pqrs.Karabiner-VirtualHIDDevice-Daemon

tmp_dir=
cleanup() {
	if [[ -n ${tmp_dir:-} && -d $tmp_dir ]]; then
		remove_path "$tmp_dir"
	fi
}
trap cleanup EXIT

install_driver() {
	tmp_dir=$(mktemp -d --tmpdir="${TMPDIR:-/tmp}" karabiner-vhid.XXXXXXXXXX)
	package_path=$tmp_dir/$package

	curl -fsSL -o "$package_path" "$url"
	if ! printf '%s  %s\n' "$sha256" "$package_path" | shasum -a 256 -c - >/dev/null; then
		die "karabiner-vhid: checksum mismatch for $package"
	fi

	installer -pkg "$package_path" -target /
}

if [[ ! -x $manager || ! -x $daemon ]]; then
	install_driver
fi

"$manager" forceActivate

launchctl bootout system "$plist" >/dev/null 2>&1 || true
render_template "$script_dir/templates/karabiner-vhid.plist.in" "$plist" \
	LABEL "$label" \
	DAEMON "$daemon"
chown root:wheel "$plist"
launchctl bootstrap system "$plist"
launchctl enable "system/$label"
launchctl kickstart -k "system/$label"
