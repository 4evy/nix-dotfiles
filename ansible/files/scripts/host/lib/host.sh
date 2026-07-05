#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_HOST_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_HOST_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/fs.sh
source -p "${BASH_SOURCE[0]%/*}" fs.sh

reload_udev_rules() {
	require_command udevadm
	udevadm control --reload-rules >/dev/null 2>&1 || true
}

trigger_udev() {
	if (($# == 0)); then
		die 'trigger_udev requires udevadm trigger arguments'
	fi

	require_command udevadm
	udevadm trigger "$@" >/dev/null 2>&1 || true
}

gsettings_available() {
	command -v gsettings >/dev/null 2>&1
}

gsettings_writable() {
	if (($# != 2)); then
		die 'gsettings_writable requires SCHEMA KEY'
	fi

	gsettings_available &&
		[[ $(gsettings writable "$1" "$2" 2>/dev/null || true) == true ]]
}

gsettings_set_if_writable() {
	if (($# != 3)); then
		die 'gsettings_set_if_writable requires SCHEMA KEY VALUE'
	fi

	if gsettings_writable "$1" "$2"; then
		gsettings set "$1" "$2" "$3"
	fi
}

DOTFILES_HOST_BASH_PRELUDE=$(
	printf '%s\n' 'set -euo pipefail'
	declare -f \
		die \
		require_bash_version \
		require_octal_mode \
		require_command \
		require_file \
		require_dir \
		require_safe_path \
		require_path_component \
		remove_path \
		ensure_dir \
		ensure_dirs \
		ensure_dir_mode \
		fresh_dir \
		install_file_if_changed \
		write_stdin_if_changed \
		replace_dir \
		reload_udev_rules \
		trigger_udev \
		gsettings_available \
		gsettings_writable \
		gsettings_set_if_writable
	printf '%s\n' \
		'require_bash_version 5 3' \
		'shopt -s inherit_errexit array_expand_once bash_source_fullpath globskipdots varredir_close'
)

run_host_bash() {
	local script=${1:?host Bash script is required}
	shift

	run_host bash -c "$DOTFILES_HOST_BASH_PRELUDE"$'\n'"$script" bash "$@"
}

run_host_bash_file() {
	local script_path=${1:?host Bash script path is required}
	shift

	require_file "$script_path"
	run_host_bash "$(printf 'DOTFILES_HOST_SCRIPT_FILE=%q\nDOTFILES_HOST_SCRIPT_DIR=%q\n' "$script_path" "${script_path%/*}")"$'\n'"$(<"$script_path")" "$@"
}

run_host_user_bash() {
	local script=${1:?host user Bash script is required}
	shift

	run_host_user bash -c "$DOTFILES_HOST_BASH_PRELUDE"$'\n'"$script" bash "$@"
}

run_host_user_bash_file() {
	local script_path=${1:?host user Bash script path is required}
	shift

	require_file "$script_path"
	run_host_user_bash "$(printf 'DOTFILES_HOST_SCRIPT_FILE=%q\nDOTFILES_HOST_SCRIPT_DIR=%q\n' "$script_path" "${script_path%/*}")"$'\n'"$(<"$script_path")" "$@"
}

run_host() {
	if (($# == 0)); then
		die 'run_host requires a command'
	fi

	local runner="$HOME/.local/bin/system-runner"
	require_executable "$runner"

	if [[ -f /.dockerenv || -f /run/.containerenv ]]; then
		require_command distrobox-host-exec
		distrobox-host-exec sudo -n "$runner" "$@"
	else
		if ((EUID == 0)); then
			"$@"
		else
			require_command sudo
			sudo -n "$runner" "$@"
		fi
	fi
}

run_host_user() {
	if (($# == 0)); then
		die 'run_host_user requires a command'
	fi

	if [[ -f /.dockerenv || -f /run/.containerenv ]]; then
		require_command distrobox-host-exec
		distrobox-host-exec "$@"
	else
		"$@"
	fi
}

host_has_command() {
	local command_name=${1:?command name is required}

	run_host_user_bash "command -v \"\$1\" >/dev/null 2>&1" "$command_name"
}

install_host_bin() {
	local src=${1:?source binary is required}
	local bin=${2:?binary name is required}

	require_path_component 'binary name' "$bin"
	require_executable "$src"
	run_host install -d -m 0755 /usr/local/bin
	run_host install -C -m 0755 -T -- "$src" "/usr/local/bin/$bin"
}
