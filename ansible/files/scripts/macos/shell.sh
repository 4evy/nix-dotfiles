#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh

require_arg_count 1 1 "$@"
user=$1
require_value user "$user"
require_path_component user "$user"
require_command chsh cut dscl grep tee

zsh_path=/opt/homebrew/bin/zsh
require_executable "$zsh_path"

if ! grep -Fxq -- "$zsh_path" /etc/shells; then
	printf '%s\n' "$zsh_path" | tee -a /etc/shells >/dev/null
fi

current_shell=$(dscl . -read "/Users/$user" UserShell 2>/dev/null | cut -d ' ' -f 2 || true)
if [[ $current_shell != "$zsh_path" ]]; then
	chsh -s "$zsh_path" "$user"
fi
