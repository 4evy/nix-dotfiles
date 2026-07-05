#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib host

require_arg_count 0 0 "$@"

policy_dir="$script_dir/tailscale-selinux"

run_host_bash_file "$script_dir/tailscale-bluefin-enable.host.sh"
run_host_bash_file "$script_dir/tailscale-bluefin-selinux-policy.host.sh" "$policy_dir"

if ! host_has_command tailscale; then
	printf '%s\n' 'tailscale-bluefin: tailscale is not available; add it to the Spectrum image' >&2
	exit 0
fi

tailscale_ready=0
for _ in {1..10}; do
	if run_host_user tailscale status >/dev/null 2>&1 || run_host tailscale status >/dev/null 2>&1; then
		tailscale_ready=1
		break
	fi
	sleep 1
done

if ((tailscale_ready)); then
	if ! run_host_user tailscale set --auto-update=false >/dev/null 2>&1 &&
		! run_host tailscale set --auto-update=false >/dev/null 2>&1; then
		printf '%s\n' 'tailscale-bluefin: could not disable Tailscale auto-update; keep updates managed by the Spectrum image' >&2
	fi
else
	printf '%s\n' 'tailscale-bluefin: tailscale is installed but not authenticated; run tailscale up on this host' >&2
fi

run_host_bash_file "$script_dir/tailscale-bluefin-validate-selinux.host.sh"
