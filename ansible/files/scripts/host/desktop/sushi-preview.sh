#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib host

require_arg_count 0 0 "$@"

repo_dir=$DOTFILES_REPO_ROOT
installer="$repo_dir/packages/sushi/install-sushi-preview-flatpak.sh"
require_file "$installer"

run_host_user bash "$installer"
