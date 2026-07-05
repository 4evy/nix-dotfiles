#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib cache
source_host_lib host

require_arg_count 0 0 "$@"

staging=$(fresh_host_staging_dir yaml-language-server)

install_file_if_changed "$script_dir/yaml-ls.sh" "$staging/yaml-language-server" 0755
install_host_bin "$staging/yaml-language-server" yaml-language-server
