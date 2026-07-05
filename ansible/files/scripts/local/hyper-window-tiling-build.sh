#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

package_path='packages/hyper-window-tiling'
gnome_uuid='hyper-window-tiling@4evy.local'

die() {
	printf 'hyper-window-tiling-build: %s\n' "$*" >&2
	exit 1
}

require_command() {
	local name

	for name; do
		command -v "$name" >/dev/null 2>&1 || die "required command is not available: $name"
	done
}

require_file() {
	local path=${1:?file path is required}

	[[ -f $path ]] || die "required file does not exist: $path"
}

abs_dir() {
	local path=${1:?directory path is required}

	cd -P -- "$path" 2>/dev/null && pwd -P
}

normalize_os() {
	local name=${1:?OS name is required}

	case $name in
		Linux | linux)
			printf '%s\n' linux
			;;
		Darwin | darwin | macos)
			printf '%s\n' darwin
			;;
		*)
			printf '%s\n' "$name"
			;;
	esac
}

infer_source_dir() {
	local start
	local dir

	start=$(pwd -P)
	dir=$start
	while :; do
		if [[ -f $dir/.chezmoiignore || -f $dir/.chezmoiexternal.toml || -f $dir/.chezmoiexternal.toml.tmpl || -d $dir/.chezmoiscripts ]]; then
			printf '%s\n' "$dir"
			return 0
		fi
		if [[ -d $dir/dotfiles ]] &&
			[[ -f $dir/dotfiles/.chezmoiignore || -f $dir/dotfiles/.chezmoiexternal.toml || -f $dir/dotfiles/.chezmoiexternal.toml.tmpl || -d $dir/dotfiles/.chezmoiscripts ]]; then
			printf '%s\n' "$dir/dotfiles"
			return 0
		fi
		[[ $dir != / ]] || break
		dir=${dir%/*}
		[[ -n $dir ]] || dir=/
	done

	die "could not find chezmoi source dir from $start; pass --source-dir DIR or run from this repo"
}

repo_root_from_source_dir() {
	local source_dir=${1:?source directory is required}
	local dir

	dir=$(abs_dir "$source_dir") || die "resolve chezmoi source directory: $source_dir"
	while :; do
		if [[ -f $dir/$package_path/package.json ]]; then
			printf '%s\n' "$dir"
			return 0
		fi
		[[ $dir != / ]] || break
		dir=${dir%/*}
		[[ -n $dir ]] || dir=/
	done

	die "could not find repo root containing $package_path from $source_dir"
}

install_file() {
	local src=${1:?source file is required}
	local dst=${2:?destination file is required}

	require_file "$src"
	mkdir -p -- "${dst%/*}"
	install -m 0644 -T -- "$src" "$dst"
}

stage_gnome() {
	local package_root=${1:?package root is required}
	local destination=${2:?destination is required}

	install_file "$package_root/gnome/metadata.json" "$destination/metadata.json"
	install_file "$package_root/dist/gnome/extension.js" "$destination/extension.js"
	mkdir -p -- "$destination/schemas"
	cp -a -- "$package_root/gnome/schemas/." "$destination/schemas/"
}

stage_kde() {
	local package_root=${1:?package root is required}
	local destination=${2:?destination is required}

	install_file "$package_root/kde/metadata.json" "$destination/metadata.json"
	install_file "$package_root/dist/kde/contents/code/main.js" "$destination/contents/code/main.js"
}

source_dir=${CHEZMOI_SOURCE_DIR:-}
home_dir=${CHEZMOI_HOME_DIR:-${HOME:-}}
os_name=${CHEZMOI_OS:-$(uname -s)}

while (($# > 0)); do
	case $1 in
		--version)
			printf '%s\n' 'hyper-window-tiling-build version dev'
			exit 0
			;;
		--source-dir)
			shift
			(($# > 0)) || die '--source-dir requires DIR'
			source_dir=$1
			shift
			;;
		--source-dir=*)
			source_dir=${1#--source-dir=}
			shift
			;;
		--home-dir)
			shift
			(($# > 0)) || die '--home-dir requires DIR'
			home_dir=$1
			shift
			;;
		--home-dir=*)
			home_dir=${1#--home-dir=}
			shift
			;;
		--os)
			shift
			(($# > 0)) || die '--os requires NAME'
			os_name=$1
			shift
			;;
		--os=*)
			os_name=${1#--os=}
			shift
			;;
		hyper-window-tiling-build)
			shift
			;;
		*)
			die "unexpected argument: $1"
			;;
	esac
done

if [[ $(normalize_os "$os_name") != linux ]]; then
	exit 0
fi

if [[ -z $source_dir ]]; then
	source_dir=$(infer_source_dir)
fi
[[ -n $home_dir ]] || die 'environment variable HOME is required'

require_command bun cp install mkdir mktemp mv rm

repo_root=$(repo_root_from_source_dir "$source_dir")
package_root=$repo_root/$package_path
require_file "$package_root/package.json"

(
	cd -- "$package_root"
	bun install --frozen-lockfile
	bun run build
) >&2

if [[ -n ${XDG_STATE_HOME:-} ]]; then
	state_home=$XDG_STATE_HOME
else
	state_home=$home_dir/.local/state
fi
stage_root=$state_home/dotfiles/hyper-window-tiling/build
stage_parent=${stage_root%/*}
mkdir -p -- "$stage_parent"
temp_root=$(mktemp -d "$stage_parent/.build-XXXXXX")
trap 'rm -rf -- "$temp_root"' EXIT

stage_gnome "$package_root" "$temp_root/gnome/$gnome_uuid"
stage_kde "$package_root" "$temp_root/kde/hyper-window-tiling"
rm -rf -- "$stage_root"
mv -- "$temp_root" "$stage_root"
trap - EXIT

printf '%s\n' "$stage_root"
