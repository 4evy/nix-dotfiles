#!/usr/bin/env bash
# shellcheck shell=bash

require_command systemctl

units=(
	zfs.target
	zfs-import-cache.service
	zfs-mount.service
	zfs-share.service
	zfs-zed.service
	zfs-volume-wait.service
	flatpak-preinstall.service
	NetworkManager-wait-online.service
)

disable_units=()
for unit in "${units[@]}"; do
	state=$(systemctl is-enabled "$unit" 2>/dev/null || true)
	case "$state" in
		enabled | enabled-runtime | linked | linked-runtime)
			disable_units+=("$unit")
			;;
	esac
done

if ((${#disable_units[@]} == 0)); then
	printf "%s\n" "slow-boot-services: target units are already disabled"
	exit 0
fi

systemctl disable "${disable_units[@]}"
printf "slow-boot-services: disabled %s\n" "${disable_units[*]}"
