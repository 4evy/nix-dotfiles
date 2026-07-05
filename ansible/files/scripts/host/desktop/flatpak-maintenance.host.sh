#!/usr/bin/env bash
# shellcheck shell=bash

installation=${1:?flatpak installation is required}

case "$installation" in
	user | system)
		;;
	*)
		die "flatpak-maintenance: invalid installation: $installation"
		;;
esac

repair_args=("--${installation}")
uninstall_args=("--${installation}" --unused -y)

if [[ $installation == system && ! -d /var/lib/flatpak ]]; then
	printf '%s\n' 'flatpak-maintenance: system installation is not initialized; skipping'
	exit 0
fi

printf 'flatpak-maintenance: repairing %s installation\n' "$installation"
if ! flatpak repair "${repair_args[@]}"; then
	printf 'flatpak-maintenance: repair failed for %s installation\n' "$installation" >&2
fi

printf 'flatpak-maintenance: pruning unused refs from %s installation\n' "$installation"
if ! flatpak uninstall "${uninstall_args[@]}"; then
	printf 'flatpak-maintenance: unused cleanup failed for %s installation\n' "$installation" >&2
fi
