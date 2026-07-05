#!/usr/bin/env bash
# shellcheck shell=bash
## usage: install-helix-tip-linux.sh <cache-dir> <install-prefix>
##
## Builds the upstream Helix master branch from source and installs hx plus its
## runtime into <install-prefix>.
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs

usage() {
	sed -n 's/^##[[:space:]]\{0,1\}//p' "${BASH_SOURCE[0]}"
}

if [[ ${1:-} == "-h" || ${1:-} == "--help" ]]; then
	usage
	exit 0
fi

if (($# != 2)); then
	usage >&2
	exit 2
fi

cache_dir=$1
install_prefix=$2
repo_url=https://github.com/helix-editor/helix.git
repo_dir="${cache_dir%/}/source"
build_log="${cache_dir%/}/helix-tip-build.log"
stamp_file="${install_prefix%/}/.helix-tip-revision"
checked_at_file="${install_prefix%/}/.helix-tip-checked-at"
state_version_file="${install_prefix%/}/.helix-tip-state-version"
runtime_dir="${install_prefix%/}/libexec/runtime"
check_interval_seconds=${HELIX_TIP_CHECK_INTERVAL_SECONDS:-86400}
state_version=1
helix_bin="${install_prefix%/}/bin/hx"

is_nonnegative_integer() {
	[[ ${1:-} =~ ^[0-9]+$ ]]
}

now_seconds() {
	date +%s
}

fresh_check_exists() {
	local checked_at now

	[[ -x $helix_bin ]] || return 1
	[[ -r $stamp_file ]] || return 1
	[[ -r $checked_at_file ]] || return 1
	[[ -r $state_version_file ]] || return 1
	[[ $(<"$state_version_file") == "$state_version" ]] || return 1
	is_nonnegative_integer "$check_interval_seconds" || return 1

	checked_at=$(<"$checked_at_file")
	is_nonnegative_integer "$checked_at" || return 1
	now=$(now_seconds)
	((now - checked_at < check_interval_seconds))
}

write_check_state() {
	local revision=${1:?revision is required}
	local now

	now=$(now_seconds)
	printf '%s\n' "$revision" | write_stdin_if_changed "$stamp_file" 0644
	printf '%s\n' "$now" | write_stdin_if_changed "$checked_at_file" 0644
	printf '%s\n' "$state_version" | write_stdin_if_changed "$state_version_file" 0644
}

require_command cargo date git
ensure_dirs "$cache_dir" "$install_prefix"

if fresh_check_exists; then
	printf 'Helix tip was checked less than %s seconds ago; skipping.\n' "$check_interval_seconds"
	exit 0
fi

if [[ -d "${repo_dir}/.git" ]]; then
	git -C "$repo_dir" fetch --depth=1 --prune origin +refs/heads/master:refs/remotes/origin/master
else
	remove_path "$repo_dir"
	git clone --branch master --single-branch --depth=1 "$repo_url" "$repo_dir"
fi

git -C "$repo_dir" checkout --force origin/master
revision=$(git -C "$repo_dir" rev-parse HEAD)

if [[ -x $helix_bin && -r $stamp_file ]]; then
	installed_revision=$(<"$stamp_file")
	if [[ $installed_revision == "$revision" ]]; then
		write_check_state "$revision"
		printf 'Helix tip already current at %s.\n' "$revision"
		exit 0
	fi
fi

remove_path "$runtime_dir"
ensure_dir "${runtime_dir%/*}"

if ! (
	cd "$repo_dir"
	export HELIX_DEFAULT_RUNTIME="$runtime_dir"
	cargo install \
		--path helix-term \
		--locked \
		--profile opt \
		--force \
		--root "$install_prefix"
	remove_path runtime/grammars/sources
	replace_dir runtime "$runtime_dir"
) >"$build_log" 2>&1; then
	printf 'error: Helix build failed; tail of %s follows.\n' "$build_log" >&2
	tail -n 160 "$build_log" >&2 || true
	exit 1
fi

write_check_state "$revision"
printf 'Installed Helix tip %s into %s.\n' "$revision" "$install_prefix"
