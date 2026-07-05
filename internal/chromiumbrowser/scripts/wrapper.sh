#!/usr/bin/env bash
# shellcheck shell=bash

set -Eeuo pipefail
unset DESKTOP_STARTUP_ID STARTUP_NOTIFICATION_ID XDG_ACTIVATION_TOKEN
unset FONTCONFIG_SYSROOT
export FONTCONFIG_FILE="${FONTCONFIG_FILE:-/etc/fonts/fonts.conf}"
export FONTCONFIG_PATH="${FONTCONFIG_PATH:-/etc/fonts}"
case ":${XDG_DATA_DIRS:-}:" in
	*:/usr/share:* | *:/usr/share/:*) ;;
	*) export XDG_DATA_DIRS="${XDG_DATA_DIRS:+$XDG_DATA_DIRS:}/usr/local/share:/usr/share" ;;
esac

runtime_flags=()

append_flags_file() {
	local file=${1:?flags file is required}
	[[ -r "$file" ]] || return 0

	local parsed_flags
	local -a file_flags=()

	if ! parsed_flags=$(sed '/^[[:space:]]*#/d;/^[[:space:]]*$/d' "$file" | xargs -r printf '%s\n'); then
		printf 'helium-browser: failed to parse flags file: %s\n' "$file" >&2
		return 1
	fi

	[[ -n $parsed_flags ]] || return 0
	mapfile -t file_flags <<<"$parsed_flags"
	runtime_flags+=("${file_flags[@]}")
}

XDG_CONFIG_HOME=${XDG_CONFIG_HOME:-"$HOME/.config"}
append_flags_file "$XDG_CONFIG_HOME/helium-flags.conf"

exec __COMMAND__ "${runtime_flags[@]}" "$@"
