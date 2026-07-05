#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_GNOME_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_GNOME_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/host.sh
source -p "${BASH_SOURCE[0]%/*}" host.sh

host_gnome_shell_available() {
	run_host_user_bash 'command -v gnome-shell >/dev/null 2>&1 && command -v gnome-extensions >/dev/null 2>&1'
}

host_gnome_shell_major_version() {
	run_host_user_bash_file "$DOTFILES_HOST_LIB_DIR/gnome-version.host.sh"
}

host_enable_gnome_extensions() {
	if (($# == 0)); then
		return
	fi

	run_host_user_bash_file "$DOTFILES_HOST_LIB_DIR/gnome-enable.host.sh" "$@"
}
