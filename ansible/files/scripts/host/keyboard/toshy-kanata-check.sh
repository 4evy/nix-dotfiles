#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_dir=$(cd -P -- "$script_dir/../../../../.." && pwd -P)
repo_kanata_config="$repo_dir/dotfiles/dot_config/kanata/kanata.kbd"
host_kanata_config=/etc/kanata/kanata.kbd

failures=0

fail() {
	printf 'FAIL: %s\n' "$*" >&2
	failures=$((failures + 1))
}

ok() {
	printf 'OK: %s\n' "$*"
}

warn() {
	printf 'WARN: %s\n' "$*" >&2
}

check_system_service_active() {
	local unit=${1:?unit is required}

	if systemctl is-active --quiet "$unit"; then
		ok "$unit is active"
	else
		fail "$unit is not active"
	fi
}

check_user_service_active() {
	local unit=${1:?unit is required}

	if systemctl --user is-active --quiet "$unit"; then
		ok "$unit is active"
	else
		fail "$unit is not active"
	fi
}

require_command() {
	local command_name

	for command_name; do
		if ! command -v "$command_name" >/dev/null 2>&1; then
			fail "required command is not available: $command_name"
		fi
	done
}

require_command systemctl readlink grep cmp
if ((failures > 0)); then
	exit 1
fi

check_system_service_active kanata-main.service
check_user_service_active toshy-config.service

if systemctl cat kanata-main.service 2>/dev/null |
	grep -Fq 'PrivateUsers=true'; then
	fail "kanata-main.service uses PrivateUsers=true, which can hide input/uinput supplemental groups"
else
	ok "kanata-main.service does not isolate host input/uinput groups with PrivateUsers"
fi

if [[ -f $repo_kanata_config && -f $host_kanata_config ]]; then
	if cmp -s "$repo_kanata_config" "$host_kanata_config"; then
		ok "$host_kanata_config matches the dotfiles Kanata config"
	else
		fail "$host_kanata_config does not match $repo_kanata_config"
	fi
else
	fail "Kanata config is missing from dotfiles or /etc/kanata"
fi

if [[ -L /run/kanata-main/main ]]; then
	kanata_target=$(readlink -f /run/kanata-main/main)
	case "$kanata_target" in
		/dev/input/event*)
			ok "/run/kanata-main/main points to $kanata_target"
			;;
		*)
			fail "/run/kanata-main/main points to unexpected target: $kanata_target"
			;;
	esac
else
	fail "/run/kanata-main/main is missing or is not a symlink"
fi

toshy_config="${XDG_CONFIG_HOME:-$HOME/.config}/toshy/toshy_config.py"
if [[ -f $toshy_config ]]; then
	if grep -Fq 'SLICE_MARK_START: keymapper_api' "$toshy_config" &&
		grep -Fq 'SLICE_MARK_START: kbtype_override' "$toshy_config" &&
		grep -Fq 'DOTFILES_TOSHY_ONLY_DEVICES' "$toshy_config" &&
		grep -Fq '/run/kanata-main/main' "$toshy_config" &&
		grep -Fq 'dotfiles-kanata-main' "$toshy_config"; then
		ok "Toshy config includes dotfiles Kanata device slice"
	else
		fail "Toshy config does not include the dotfiles Kanata device slice"
	fi
else
	fail "Toshy config is missing: $toshy_config"
fi

if systemctl --user cat toshy-config.service 2>/dev/null |
	grep -Fxq 'Environment=DOTFILES_TOSHY_ONLY_DEVICES=/run/kanata-main/main'; then
	ok "toshy-config.service restricts devices to /run/kanata-main/main"
else
	fail "toshy-config.service is missing the Kanata-only device drop-in"
fi

if systemctl --user cat toshy-config.service 2>/dev/null |
	grep -Fq 'waiting for /run/kanata-main/main timed out'; then
	ok "toshy-config.service waits for the Kanata virtual device"
else
	fail "toshy-config.service does not wait for the Kanata virtual device"
fi

if systemctl --user is-enabled --quiet toshy-kanata-device.path 2>/dev/null; then
	ok "toshy-kanata-device.path is enabled"
else
	fail "toshy-kanata-device.path is not enabled"
fi

if systemctl --user cat toshy-kanata-device.path 2>/dev/null |
	grep -Fxq 'PathChanged=/run/kanata-main/main'; then
	ok "toshy-kanata-device.path watches the Kanata virtual device"
else
	fail "toshy-kanata-device.path is missing the Kanata virtual device watch"
fi

if systemctl --user cat toshy-kanata-device.service 2>/dev/null |
	grep -Fq 'reset-failed toshy-config.service' &&
	systemctl --user cat toshy-kanata-device.service 2>/dev/null |
	grep -Fq '[ -e /run/kanata-main/main ]' &&
	systemctl --user cat toshy-kanata-device.service 2>/dev/null |
	grep -Fq 'restart toshy-config.service'; then
	ok "toshy-kanata-device.service can recover a failed Toshy service"
else
	fail "toshy-kanata-device.service does not recover failed Toshy starts"
fi

if systemctl cat input-remapper.service >/dev/null 2>&1; then
	if [[ ${DOTFILES_KEEP_INPUT_REMAPPER:-0} == 1 ]]; then
		warn "input-remapper.service exists and keep flag is set; skipping conflict check"
	elif systemctl is-active --quiet input-remapper.service; then
		fail "input-remapper.service is active and can compete with Kanata/Toshy"
	else
		ok "input-remapper.service is not active"
	fi
else
	ok "input-remapper.service is not installed"
fi

if ((failures > 0)); then
	printf 'toshy-kanata-check: %d check(s) failed\n' "$failures" >&2
	exit 1
fi

printf '%s\n' 'toshy-kanata-check: setup looks consistent'
