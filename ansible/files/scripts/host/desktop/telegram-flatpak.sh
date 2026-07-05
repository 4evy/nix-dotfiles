#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib host

require_arg_count 0 0 "$@"

if ! host_has_command flatpak; then
	printf '%s\n' 'telegram-flatpak: flatpak is not available; skipping'
	exit 0
fi

run_host_user_bash_file "$script_dir/telegram-flatpak.host.sh"
