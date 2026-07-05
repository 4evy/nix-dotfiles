#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_HYPER_WINDOW_TILING_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_HYPER_WINDOW_TILING_SOURCED=1

hyper_window_tiling_build() {
	local source_dir=${1:?chezmoi source directory is required}
	local local_builder=${HOME:-}/.local/bin/hyper-window-tiling-build
	local repo_builder=$DOTFILES_REPO_ROOT/ansible/files/scripts/local/hyper-window-tiling-build.sh

	if command -v hyper-window-tiling-build >/dev/null 2>&1; then
		hyper-window-tiling-build --source-dir "$source_dir"
		return
	fi

	if [[ -n ${HOME:-} && -x $local_builder ]]; then
		"$local_builder" --source-dir "$source_dir"
		return
	fi

	if [[ -x $repo_builder ]]; then
		"$repo_builder" --source-dir "$source_dir"
		return
	fi

	die 'required command is not available: hyper-window-tiling-build'
}
