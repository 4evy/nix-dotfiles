#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_PATHS_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_PATHS_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/core.sh
source -p "${BASH_SOURCE[0]%/*}" core.sh

require_safe_path() {
	local path=${1:?path is required}

	case "$path" in
		"" | "/" | "//" | "." | ".." | */. | */..)
			die "refusing unsafe path: $path"
			;;
	esac
}

require_path_component() {
	local name=${1:?component name is required}
	local value=${2-}

	case "$value" in
		"" | "." | ".." | */*)
			die "$name must be a single path component"
			;;
	esac
}
