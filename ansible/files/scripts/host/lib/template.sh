#!/usr/bin/env bash
# shellcheck shell=bash

if [[ ${DOTFILES_HOST_LIB_TEMPLATE_SOURCED:-0} == 1 ]]; then
	return 0
fi
DOTFILES_HOST_LIB_TEMPLATE_SOURCED=1

# shellcheck source=ansible/files/scripts/host/lib/fs.sh
source -p "${BASH_SOURCE[0]%/*}" fs.sh

render_template() {
	local source=${1:?template source is required}
	local destination=${2:?template destination is required}
	shift 2

	render_template_mode 0644 "$source" "$destination" "$@"
}

render_template_mode() {
	local mode=${1:?template mode is required}
	local source=${2:?template source is required}
	local destination=${3:?template destination is required}
	shift 3

	if (($# % 2 != 0)); then
		die 'template replacements must be NAME VALUE pairs'
	fi

	require_octal_mode 'template mode' "$mode"
	local name
	local replacements=()
	while (($# > 0)); do
		name=$1
		if [[ ! $name =~ ^[A-Z][A-Z0-9_]*$ ]]; then
			die "invalid template replacement name: $name"
		fi
		replacements+=("$1" "$2")
		shift 2
	done

	require_file "$source"
	require_command awk
	awk '
function escape_replacement(value) {
	gsub(/\\/, "\\\\", value)
	gsub(/&/, "\\\\&", value)
	return value
}
BEGIN {
	for (i = 2; i < ARGC; i += 2) {
		replacements["@" ARGV[i] "@"] = escape_replacement(ARGV[i + 1])
	}
	ARGC = 2
}
{
	for (placeholder in replacements) {
		gsub(placeholder, replacements[placeholder])
	}
	print
}
' "$source" "${replacements[@]}" | write_stdin_if_changed "$destination" "$mode"
}
