#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=${DOTFILES_HOST_SCRIPT_DIR:-}
if [[ -z $script_dir ]]; then
	script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
fi
awk_script="$script_dir/rustdesk-merge-options.awk"
config_file=${1:-$HOME/.config/rustdesk/RustDesk2.toml}

require_command awk
require_file "$awk_script"

config_dir=${config_file%/*}
ensure_dir "$config_dir"

input_file=/dev/null
if [[ -f $config_file ]]; then
	input_file=$config_file
fi

awk -f "$awk_script" "$input_file" | write_stdin_if_changed "$config_file" 0644
