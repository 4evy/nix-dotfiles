#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_HTTP_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_HTTP_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/core.sh
source -p "${BASH_SOURCE[0]%/*}" core.sh

curl_download() {
	local url=${1:?download URL is required}
	local dest=${2:?download destination is required}
	local -a curl_args=(-fsSL --proto '=https' --proto-redir '=https' --retry 3 --retry-delay 1 --retry-connrefused)

	require_command curl
	curl "${curl_args[@]}" -o "$dest" "$url"
}

curl_stdout() {
	local url=${1:?download URL is required}
	local -a curl_args=(-fsSL --proto '=https' --proto-redir '=https' --retry 3 --retry-delay 1 --retry-connrefused)

	require_command curl
	curl "${curl_args[@]}" "$url"
}
