#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

policy_dir=${1:?policy directory is required}
policy_makefile=/usr/share/selinux/devel/Makefile

if ! command -v getenforce >/dev/null 2>&1 || [[ $(getenforce 2>/dev/null || true) == Disabled ]]; then
	exit 0
fi

if [[ ! -f $policy_makefile ]]; then
	printf "%s\n" "tailscale-bluefin: selinux-policy-devel is not installed; add it to the Spectrum image to build the tailscaled SELinux policy" >&2
	exit 0
fi

for file in tailscaled.te tailscaled.fc tailscaled.if; do
	if [[ ! -f $policy_dir/$file ]]; then
		die "tailscale-bluefin: missing SELinux policy source: $policy_dir/$file"
	fi
done

require_command checkmodule cut grep head install jq make pgrep ps semodule semodule_package sha256sum restorecon systemctl

policy_hash=$(
	cd "$policy_dir"
	sha256sum tailscaled.te tailscaled.fc tailscaled.if | sha256sum | cut -d " " -f 1
)
policy_hash_file=/var/lib/tailscale/dotfiles-selinux-policy.sha256

policy_installed=0
if semodule -l 2>/dev/null | grep -Eq "^tailscaled([[:space:]]|$)"; then
	policy_installed=1
fi

install_policy=0
if ((policy_installed == 0)) || [[ ! -f $policy_hash_file ]] || [[ $(<"$policy_hash_file") != "$policy_hash" ]]; then
	install_policy=1
fi

dropin_dir=/etc/systemd/system/tailscaled.service.d
dropin_file=$dropin_dir/10-selinux-context.conf
printf -v desired_dropin "%s\n%s" "[Service]" "SELinuxContext=system_u:system_r:tailscaled_t:s0"

tailscaled_service_context() {
	local main_pid

	main_pid=$(systemctl show -P MainPID tailscaled 2>/dev/null || true)
	if [[ -n $main_pid && $main_pid != 0 ]]; then
		ps -p "$main_pid" -o label= 2>/dev/null | head -n 1
	fi
}

tailscale_ssh_sessions_active() {
	local main_pid

	main_pid=$(systemctl show -P MainPID tailscaled 2>/dev/null || true)
	if [[ -z $main_pid || $main_pid == 0 ]]; then
		return 1
	fi

	pgrep -P "$main_pid" -f "tailscaled be-child ssh" >/dev/null 2>&1
}

install_dropin=0
if [[ ! -f $dropin_file ]] || [[ $(<"$dropin_file") != "$desired_dropin" ]]; then
	install_dropin=1
fi

if ((install_policy || install_dropin)) &&
	[[ ${DOTFILES_TAILSCALE_ALLOW_LIVE_RELOAD:-0} != 1 ]] &&
	tailscale_ssh_sessions_active; then
	printf "%s\n" "tailscale-bluefin: active Tailscale SSH session detected; deferring SELinux policy/drop-in changes to avoid interrupting it" >&2
	printf "%s\n" "tailscale-bluefin: rerun locally, after disconnecting SSH, or set DOTFILES_TAILSCALE_ALLOW_LIVE_RELOAD=1 to force it" >&2
	exit 0
fi

if ((install_policy)); then
	build_dir=$(mktemp -d)
	trap 'remove_path "$build_dir"' EXIT
	for file in tailscaled.te tailscaled.fc tailscaled.if; do
		install_file_if_changed "$policy_dir/$file" "$build_dir/$file" 0644
	done
	make -C "$build_dir" -f "$policy_makefile" tailscaled.pp >/dev/null
	semodule -i "$build_dir/tailscaled.pp"

	ensure_dir_mode 0700 /var/lib/tailscale
	printf "%s\n" "$policy_hash" | write_stdin_if_changed "$policy_hash_file" 0644

	for path in \
		/usr/bin/tailscaled \
		/usr/sbin/tailscaled \
		/usr/lib/systemd/system/tailscaled.service \
		/etc/systemd/system/tailscaled.service; do
		if [[ -e $path ]]; then
			restorecon "$path" || true
		fi
	done

	for path in /var/lib/tailscale /var/cache/tailscale /run/tailscale /var/run/tailscale; do
		if [[ -e $path ]]; then
			restorecon -R "$path" || true
		fi
	done
fi

if ((install_dropin)); then
	printf "%s\n" "$desired_dropin" | write_stdin_if_changed "$dropin_file" 0644
	systemctl daemon-reload
fi

current_context=$(tailscaled_service_context)
restart_required=0
if ! systemctl is-active --quiet tailscaled; then
	restart_required=1
elif ((install_dropin)); then
	restart_required=1
elif [[ $current_context != system_u:system_r:tailscaled_t:s0 ]]; then
	restart_required=1
fi

if ((restart_required)) &&
	[[ ${DOTFILES_TAILSCALE_ALLOW_LIVE_RELOAD:-0} != 1 ]] &&
	tailscale_ssh_sessions_active; then
	printf "%s\n" "tailscale-bluefin: active Tailscale SSH session detected; deferring tailscaled restart to avoid interrupting it" >&2
	printf "%s\n" "tailscale-bluefin: rerun locally, after disconnecting SSH, or set DOTFILES_TAILSCALE_ALLOW_LIVE_RELOAD=1 to force it" >&2
	exit 0
fi

if ((restart_required)) && ! systemctl restart tailscaled; then
	rm -f -- /etc/systemd/system/tailscaled.service.d/10-selinux-context.conf
	systemctl daemon-reload
	systemctl reset-failed tailscaled || true
	systemctl start tailscaled || true
	die "tailscale-bluefin: confined tailscaled restart failed; removed SELinuxContext drop-in and restarted the unconfined service"
fi

current_context=$(tailscaled_service_context)
if [[ $current_context != system_u:system_r:tailscaled_t:s0 ]]; then
	die "tailscale-bluefin: tailscaled is not running in the expected SELinux context; got: ${current_context:-not running}"
fi
