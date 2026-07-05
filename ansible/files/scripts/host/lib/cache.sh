#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_CACHE_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_CACHE_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/fs.sh
source -p "${BASH_SOURCE[0]%/*}" fs.sh

fresh_host_staging_dir() {
	local name=${1:?tool name is required}
	local staging
	require_path_component 'tool name' "$name"
	staging="$HOME/.local/share/dotfiles/host/bin-staging/$name"

	fresh_dir "$staging"
	printf '%s\n' "$staging"
}

dotfiles_cache_dir() {
	local name=${1:?cache name is required}
	require_path_component 'cache name' "$name"

	printf '%s\n' "${XDG_CACHE_HOME:-$HOME/.cache}/dotfiles/$name"
}
