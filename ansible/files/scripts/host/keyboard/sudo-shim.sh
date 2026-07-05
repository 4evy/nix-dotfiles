#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

if [[ ${1-} == "-k" ]]; then
	exit 0
fi
if [[ ${1-} == "-n" ]]; then
	shift
fi

sudo_bin=${TOSHY_SUDO:-}
if [[ -z $sudo_bin ]]; then
	shim_path=$(readlink -f -- "$0")
	while IFS= read -r candidate; do
		candidate=$(readlink -f -- "$candidate" 2>/dev/null) || continue
		if [[ $candidate != "$shim_path" ]]; then
			sudo_bin=$candidate
			break
		fi
	done < <(type -P -a sudo 2>/dev/null || true)
fi
if [[ -z $sudo_bin ]]; then
	printf '%s\n' 'toshy-kanata-chain: sudo is not available on PATH' >&2
	exit 127
fi

exec "$sudo_bin" -n "$@"
