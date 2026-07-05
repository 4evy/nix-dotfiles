#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_JSON_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_JSON_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/core.sh
source -p "${BASH_SOURCE[0]%/*}" core.sh

jq_read_text() {
	local filter=${1:?jq filter is required}
	local input=${2-}

	require_command jq
	jq -er "$filter" <<<"$input"
}
