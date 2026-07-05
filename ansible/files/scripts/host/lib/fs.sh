#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_FS_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_FS_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/paths.sh
source -p "${BASH_SOURCE[0]%/*}" paths.sh

remove_path() {
	local path=${1:?path is required}

	require_safe_path "$path"
	require_command rm
	rm -rf --one-file-system --preserve-root=all -- "$path"
}

ensure_dir() {
	local dir=${1:?directory path is required}

	require_safe_path "$dir"
	require_command mkdir
	mkdir -p -- "$dir"
}

ensure_dirs() {
	local dir

	if (($# == 0)); then
		die 'directory path is required'
	fi

	for dir; do
		ensure_dir "$dir"
	done
}

ensure_dir_mode() {
	local mode=${1:?directory mode is required}
	local dir
	shift

	require_octal_mode 'directory mode' "$mode"
	if (($# == 0)); then
		die 'directory path is required'
	fi

	require_command install
	for dir; do
		require_safe_path "$dir"
		install -d -m "$mode" -- "$dir"
	done
}

fresh_dir() {
	local dir=${1:?directory path is required}

	remove_path "$dir"
	ensure_dir "$dir"
}

install_file_if_changed() {
	local src=${1:?source file is required}
	local dest=${2:?destination file is required}
	local mode=${3:-0644}

	require_octal_mode 'file mode' "$mode"

	require_file "$src"
	require_safe_path "$dest"
	require_command install
	install -D -C -m "$mode" -T -- "$src" "$dest"
}

write_stdin_if_changed() {
	local path=${1:?destination path is required}
	local mode=${2:-0644}
	local dir tmp

	require_octal_mode 'file mode' "$mode"

	require_safe_path "$path"
	require_command cat install mktemp rm

	dir=${path%/*}
	[[ $dir != "$path" ]] || dir=.
	ensure_dir "$dir"

	tmp=$(mktemp --tmpdir="$dir" ".${path##*/}.XXXXXX")
	if ! cat >"$tmp"; then
		rm -f -- "$tmp"
		return 1
	fi

	if ! install -D -C -m "$mode" -T -- "$tmp" "$path"; then
		rm -f -- "$tmp"
		return 1
	fi

	rm -f -- "$tmp"
}

replace_dir() {
	local src=${1:?source directory is required}
	local dest=${2:?destination directory is required}
	local parent

	require_dir "$src"
	require_safe_path "$dest"
	require_command cp

	parent=${dest%/*}
	[[ $parent != "$dest" ]] || parent=.
	ensure_dir "$parent"

	remove_path "$dest"
	cp -aT --reflink=auto --no-preserve=ownership -- "$src" "$dest"
}
