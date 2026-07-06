#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

cd "$script_dir"
if [[ -n "${UV_PROJECT_ENVIRONMENT:-}" && ! -f "${UV_PROJECT_ENVIRONMENT}/pyvenv.cfg" ]]; then
	uv venv --system-site-packages "$UV_PROJECT_ENVIRONMENT"
fi

exec uv run --locked spectrum-build "$@"
