#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

home_dir=${CHEZMOI_HOME_DIR:-${HOME:?HOME is required}}
cache_home=${XDG_CACHE_HOME:-"$home_dir/.cache"}

write_if_changed() {
	local target=${1:?target path is required}
	local source=${2:?source path is required}

	mkdir -p "${target%/*}"
	if [[ -f $target ]] && cmp -s "$source" "$target"; then
		rm -f "$source"
		return 0
	fi
	mv "$source" "$target"
}

capture_if_available() {
	local bin=${1:?binary name is required}
	local target=${2:?target path is required}
	shift 2

	if ! command -v "$bin" >/dev/null 2>&1; then
		return 0
	fi

	local tmp
	tmp=$(mktemp)
	if "$@" >"$tmp"; then
		write_if_changed "$target" "$tmp"
	else
		printf 'failed to generate shell init for %s\n' "$bin" >&2
		rm -f "$tmp"
	fi
}

capture_fzf_zsh() {
	local target=${1:?target path is required}

	if ! command -v fzf >/dev/null 2>&1; then
		return 0
	fi

	local tmp
	tmp=$(mktemp)
	if fzf --zsh | sed \
		-e '/^### completion\.zsh ###$/,$d' \
		-e '/^  eval \$__fzf_key_bindings_options$/i\
  __fzf_key_bindings_options=${__fzf_key_bindings_options/ zle on/}\
  __fzf_key_bindings_options=${__fzf_key_bindings_options/ zle off/}' >"$tmp"; then
		write_if_changed "$target" "$tmp"
	else
		printf 'failed to generate shell init for fzf\n' >&2
		rm -f "$tmp"
	fi
}

generate_completion() {
	local shell_name=${1:?shell name is required}
	local target=${2:?target path is required}
	local bin=${3:?binary name is required}
	shift 3

	if ! command -v "$bin" >/dev/null 2>&1; then
		return 0
	fi

	local -a command=()
	while (($#)); do
		if [[ $1 == -- ]]; then
			shift
			break
		fi
		command+=("$1")
		shift
	done

	local -a suffix=("$@")

	local tmp
	tmp=$(mktemp)
	if "${command[@]}" "$shell_name" "${suffix[@]}" >"$tmp"; then
		write_if_changed "$target" "$tmp"
	else
		printf 'failed to generate completions for %s\n' "$bin" >&2
		rm -f "$tmp"
	fi
}

mkdir -p \
	"$cache_home/fzf" \
	"$cache_home/starship" \
	"$cache_home/zoxide" \
	"$cache_home/atuin" \
	"$cache_home/zsh/completions" \
	"$cache_home/bash/completions"

for shell_name in zsh bash; do
	if [[ $shell_name == zsh ]]; then
		capture_fzf_zsh "$cache_home/fzf/init.zsh"
	else
		capture_if_available fzf "$cache_home/fzf/init.bash" fzf --bash
	fi
	capture_if_available starship "$cache_home/starship/init.$shell_name" starship init "$shell_name"
	if [[ $shell_name == bash ]]; then
		capture_if_available zoxide "$cache_home/zoxide/init.$shell_name" zoxide init "$shell_name" --cmd cd
	else
		capture_if_available zoxide "$cache_home/zoxide/init.$shell_name" zoxide init "$shell_name"
	fi
	capture_if_available atuin "$cache_home/atuin/init.$shell_name" atuin init "$shell_name" --disable-up-arrow

	outdir="$cache_home/$shell_name/completions"
	prefix=
	if [[ $shell_name == zsh ]]; then
		prefix=_
	fi

	if command -v atuin >/dev/null 2>&1; then
		atuin gen-completions --shell "$shell_name" --out-dir "$outdir" ||
			printf 'failed to generate completions for atuin\n' >&2
	fi

	generate_completion "$shell_name" "$outdir/${prefix}chezmoi" chezmoi chezmoi completion
	generate_completion "$shell_name" "$outdir/${prefix}jj" jj jj util completion
	generate_completion "$shell_name" "$outdir/${prefix}zellij" zellij zellij setup --generate-completion
	generate_completion "$shell_name" "$outdir/${prefix}starship" starship starship completions
	generate_completion "$shell_name" "$outdir/${prefix}deno" deno deno completions
	generate_completion "$shell_name" "$outdir/${prefix}delta" delta delta --generate-completion
	generate_completion "$shell_name" "$outdir/${prefix}rustup" rustup rustup completions
	generate_completion "$shell_name" "$outdir/${prefix}cargo" rustup rustup completions -- cargo
done
