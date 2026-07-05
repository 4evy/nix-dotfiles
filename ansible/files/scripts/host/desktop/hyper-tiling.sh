#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh

require_arg_count 0 0 "$@"

bash "$script_dir/hyper-tiling-gnome.sh"
bash "$script_dir/hyper-tiling-kde.sh"
