#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_ENTRYPOINT_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_ENTRYPOINT_SOURCED=1

host_entrypoint_lib_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

export DOTFILES_HOST_SCRIPT_ROOT
DOTFILES_HOST_SCRIPT_ROOT=$(cd -P -- "$host_entrypoint_lib_dir/.." && pwd -P)
export DOTFILES_REPO_ROOT
DOTFILES_REPO_ROOT=$(cd -P -- "$DOTFILES_HOST_SCRIPT_ROOT/../../../.." && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/core.sh
source -p "$host_entrypoint_lib_dir" core.sh
