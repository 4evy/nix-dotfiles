#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

default_runner_path='/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/sbin:/usr/local/bin:/run/wrappers/bin:/run/current-system/sw/bin:/nix/var/nix/profiles/default/bin:/etc/profiles/per-user/root/bin'

die() {
	printf 'system-runner: %s\n' "$*" >&2
	exit 2
}

is_path_like() {
	local value=${1:?value is required}

	[[ $value == */* ]]
}

find_in_path() {
	local name=${1:?command name is required}
	local search_path=${2-}
	local dir
	local candidate

	[[ -n $search_path ]] || return 1

	while :; do
		if [[ $search_path == *:* ]]; then
			dir=${search_path%%:*}
			search_path=${search_path#*:}
		else
			dir=$search_path
			search_path=
		fi

		if [[ -n $dir ]]; then
			candidate=$dir/$name
		else
			candidate=$name
		fi
		if [[ -f $candidate && -x $candidate ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi

		[[ -n $search_path ]] || break
	done

	return 1
}

env_overrides=()
while (($# > 0)); do
	case $1 in
		--version)
			printf '%s\n' 'system-runner version dev'
			exit 0
			;;
		--env)
			shift
			(($# > 0)) || die '--env requires KEY=VALUE'
			env_overrides+=("$1")
			shift
			;;
		--env=*)
			env_overrides+=("${1#--env=}")
			shift
			;;
		--)
			shift
			break
			;;
		-*)
			die "unknown flag: $1"
			;;
		*)
			break
			;;
	esac
done

(($# > 0)) || die 'expected COMMAND [ARG...]'

path_env=$default_runner_path
for value in "${env_overrides[@]}"; do
	if [[ $value != *=* ]]; then
		die '--env requires KEY=VALUE'
	fi
	key=${value%%=*}
	if [[ -z $key ]]; then
		die '--env variable name must not be empty'
	fi
	if [[ $key == PATH ]]; then
		path_env=${value#*=}
	fi
done

program=$1
shift
if is_path_like "$program"; then
	if [[ -e $program && ! -d $program && ! -x $program ]]; then
		set -- "$program" "$@"
		program=/bin/cat
	fi
else
	if resolved=$(find_in_path "$program" "$path_env"); then
		program=$resolved
	fi
fi

exec env "${env_overrides[@]}" "PATH=$path_env" "$program" "$@"
