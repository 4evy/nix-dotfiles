#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib fs
source_host_lib http

require_arg_count 0 0 "$@"
require_command fc-match unzip mktemp install find grep

font_family="JetBrainsMono Nerd Font Mono"
font_dir="${XDG_DATA_HOME:-$HOME/.local/share}/fonts/JetBrainsMonoNerdFont"
archive_url="https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.zip"

if fc-match "$font_family" | grep -Fq "JetBrainsMonoNerdFontMono"; then
	exit 0
fi

tmp_dir=$(mktemp -d)
cleanup() {
	remove_path "$tmp_dir"
}
trap cleanup EXIT

archive="$tmp_dir/JetBrainsMono.zip"
extract_dir="$tmp_dir/extract"

curl_download "$archive_url" "$archive"
ensure_dir "$extract_dir"
unzip -q "$archive" -d "$extract_dir"

ensure_dir_mode 0755 "$font_dir"
find "$extract_dir" -maxdepth 1 -type f \( -name 'JetBrainsMonoNerdFontMono-*.ttf' -o -name 'JetBrainsMonoNLNerdFontMono-*.ttf' \) \
	-exec install -C -m 0644 -t "$font_dir" -- {} +

if ! find "$font_dir" -maxdepth 1 -type f \
	\( -name 'JetBrainsMonoNerdFontMono-*.ttf' -o -name 'JetBrainsMonoNLNerdFontMono-*.ttf' \) \
	-print -quit | grep -q .; then
	die "JetBrainsMono Nerd Font archive did not contain expected Mono font files"
fi

if command -v fc-cache >/dev/null 2>&1; then
	fc-cache -f "$font_dir"
fi

if ! fc-match "$font_family" | grep -Fq "JetBrainsMonoNerdFontMono"; then
	die "fontconfig did not resolve $font_family to JetBrainsMono Nerd Font Mono"
fi
