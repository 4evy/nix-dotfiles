#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

if command -v getenforce >/dev/null 2>&1 &&
	[[ $(getenforce 2>/dev/null || true) == Enforcing ]] &&
	command -v tailscale >/dev/null 2>&1 &&
	tailscale debug prefs 2>/dev/null | jq -e ".RunSSH == true" >/dev/null; then
	main_pid=$(systemctl show -P MainPID tailscaled 2>/dev/null || true)
	context=
	if [[ -n $main_pid && $main_pid != 0 ]]; then
		context=$(ps -p "$main_pid" -o label= 2>/dev/null | head -n 1)
	fi

	if [[ $context != system_u:system_r:tailscaled_t:s0 ]]; then
		die "tailscale-bluefin: Tailscale SSH is enabled under enforcing SELinux, but tailscaled is running as ${context:-not running}"
	fi

	printf "%s\n" "tailscale-bluefin: Tailscale SSH SELinux policy is installed; tailscale status may still show the upstream generic SELinux warning" >&2
fi
