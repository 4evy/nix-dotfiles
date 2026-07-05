#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib host

require_arg_count 0 0 "$@"

selinux_policy_dir="$script_dir/rustdesk-selinux"

remove_rustdesk_flatpak() {
	if ! host_has_command flatpak; then
		return 0
	fi

	run_host_user flatpak uninstall --user --noninteractive com.rustdesk.RustDesk >/dev/null 2>&1 || true
	run_host flatpak uninstall --system --noninteractive com.rustdesk.RustDesk >/dev/null 2>&1 || true
}

install_rustdesk_apparmor_profile() {
	run_host_bash "$(
		cat <<'HOST_BASH'
set -euo pipefail

if [[ ${DOTFILES_RUSTDESK_APPARMOR:-1} == 0 ]]; then
  exit 0
fi

if ! test -r /sys/module/apparmor/parameters/enabled ||
  ! grep -qi '^Y' /sys/module/apparmor/parameters/enabled ||
  ! command -v apparmor_parser >/dev/null 2>&1; then
  exit 0
fi

profile_file=/etc/apparmor.d/dotfiles-rustdesk
write_stdin_if_changed "$profile_file" 0644 <<'APPARMOR_PROFILE'
abi <abi/4.0>,
include <tunables/global>

profile dotfiles-rustdesk /usr/bin/rustdesk flags=(default_allow) {
  capability,
  network,
  dbus,
  unix,
  signal,
  ptrace,

  /dev/uinput rw,
  /dev/input/** rw,
  /run/user/*/bus rw,
  /run/user/*/pipewire-0 rw,
  /run/user/*/wayland-* rw,
  /tmp/.X11-unix/* rw,
  /usr/share/rustdesk/** rix,

  owner @{HOME}/.config/rustdesk/** rwk,
  owner @{HOME}/.local/share/rustdesk/** rwk,

  include if exists <local/dotfiles-rustdesk>
}

profile dotfiles-rustdesk-share /usr/share/rustdesk/rustdesk flags=(default_allow) {
  capability,
  network,
  dbus,
  unix,
  signal,
  ptrace,

  /dev/uinput rw,
  /dev/input/** rw,
  /run/user/*/bus rw,
  /run/user/*/pipewire-0 rw,
  /run/user/*/wayland-* rw,
  /tmp/.X11-unix/* rw,
  /usr/bin/rustdesk rix,
  /usr/share/rustdesk/** rix,

  owner @{HOME}/.config/rustdesk/** rwk,
  owner @{HOME}/.local/share/rustdesk/** rwk,

  include if exists <local/dotfiles-rustdesk-share>
}
APPARMOR_PROFILE

apparmor_parser -r "$profile_file"
HOST_BASH
	)"
}

install_rustdesk_selinux_policy() {
	run_host_bash "$(
		cat <<'HOST_BASH'
set -euo pipefail

policy_dir=${1:?policy directory is required}
policy_makefile=/usr/share/selinux/devel/Makefile

if [[ ${DOTFILES_RUSTDESK_SELINUX:-1} == 0 ]]; then
  exit 0
fi

if ! command -v getenforce >/dev/null 2>&1 || [[ $(getenforce 2>/dev/null || true) == Disabled ]]; then
  exit 0
fi

if [[ ! -f $policy_makefile ]]; then
  printf "%s\n" "rustdesk-tailscale: selinux-policy-devel is not installed; add it to the Spectrum image to build the RustDesk SELinux policy" >&2
  exit 0
fi

for file in rustdesk.te rustdesk.fc rustdesk.if; do
  if [[ ! -f $policy_dir/$file ]]; then
    die "rustdesk-tailscale: missing SELinux policy source: $policy_dir/$file"
  fi
done

require_command cut grep head install make ps semodule sha256sum restorecon systemctl

policy_hash=$(
  cd "$policy_dir"
  sha256sum rustdesk.te rustdesk.fc rustdesk.if | sha256sum | cut -d " " -f 1
)
policy_hash_file=/var/lib/rustdesk/dotfiles-selinux-policy.sha256

policy_installed=0
if semodule -l 2>/dev/null | grep -Eq "^rustdesk([[:space:]]|$)"; then
  policy_installed=1
fi

install_policy=0
if ((policy_installed == 0)) || [[ ! -f $policy_hash_file ]] || [[ $(<"$policy_hash_file") != "$policy_hash" ]]; then
  install_policy=1
fi

dropin_dir=/etc/systemd/system/rustdesk.service.d
dropin_file=$dropin_dir/10-selinux-context.conf
printf -v desired_dropin "%s\n%s" "[Service]" "SELinuxContext=system_u:system_r:rustdesk_t:s0"

install_dropin=0
if [[ ! -f $dropin_file ]] || [[ $(<"$dropin_file") != "$desired_dropin" ]]; then
  install_dropin=1
fi

if ((install_policy)); then
  build_dir=$(mktemp -d)
  trap 'remove_path "$build_dir"' EXIT
  for file in rustdesk.te rustdesk.fc rustdesk.if; do
    install_file_if_changed "$policy_dir/$file" "$build_dir/$file" 0644
  done
  make -C "$build_dir" -f "$policy_makefile" rustdesk.pp >/dev/null
  semodule -i "$build_dir/rustdesk.pp"

  ensure_dir_mode 0700 /var/lib/rustdesk
  printf "%s\n" "$policy_hash" | write_stdin_if_changed "$policy_hash_file" 0644

  for path in \
    /usr/bin/rustdesk \
    /usr/share/rustdesk/rustdesk \
    /etc/systemd/system/rustdesk.service \
    /usr/lib/systemd/system/rustdesk.service \
    /var/lib/rustdesk \
    /run/rustdesk.pid \
    /var/run/rustdesk.pid; do
    if [[ -e $path ]]; then
      restorecon -R "$path" || true
    fi
  done
fi

if ((install_dropin)); then
  ensure_dir "$dropin_dir"
  printf "%s\n" "$desired_dropin" | write_stdin_if_changed "$dropin_file" 0644
  systemctl daemon-reload
fi

rustdesk_service_context() {
  local main_pid

  main_pid=$(systemctl show -P MainPID rustdesk.service 2>/dev/null || true)
  if [[ -n $main_pid && $main_pid != 0 ]]; then
    ps -p "$main_pid" -o label= 2>/dev/null | head -n 1
  fi
}

restart_required=0
if systemctl is-active --quiet rustdesk.service; then
  current_context=$(rustdesk_service_context)
  if ((install_policy || install_dropin)) || [[ $current_context != system_u:system_r:rustdesk_t:s0 ]]; then
    restart_required=1
  fi
fi

if ((restart_required)) && ! systemctl restart rustdesk.service; then
  rm -f -- "$dropin_file"
  systemctl daemon-reload
  systemctl reset-failed rustdesk.service || true
  systemctl start rustdesk.service || true
  die "rustdesk-tailscale: rustdesk restart failed under rustdesk_t; removed SELinuxContext drop-in and restarted unconfined"
fi

if systemctl is-active --quiet rustdesk.service; then
  current_context=$(rustdesk_service_context)
  if [[ $current_context != system_u:system_r:rustdesk_t:s0 ]]; then
    die "rustdesk-tailscale: rustdesk is not running in the expected SELinux context; got: ${current_context:-not running}"
  fi
fi
HOST_BASH
	)" "$selinux_policy_dir"
}

prepare_rustdesk_wayland_session() {
	run_host_user_bash "$(
		cat <<'HOST_BASH'
set -euo pipefail

if [[ ${XDG_SESSION_TYPE:-} != wayland ]]; then
  exit 0
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl --user reset-failed xdg-desktop-portal.service xdg-desktop-portal-gnome.service xdg-desktop-portal-gtk.service pipewire.service wireplumber.service >/dev/null 2>&1 || true
  systemctl --user start xdg-desktop-portal.service pipewire.service wireplumber.service >/dev/null 2>&1 || true
fi

if [[ -z ${XDG_RUNTIME_DIR:-} || ! -S ${XDG_RUNTIME_DIR:-}/bus ]]; then
  printf '%s\n' 'rustdesk-tailscale: Wayland session bus is not available; RustDesk portal capture will fail until the user session is healthy' >&2
fi

if [[ -n ${XDG_RUNTIME_DIR:-} && ! -S ${XDG_RUNTIME_DIR}/pipewire-0 ]]; then
  printf '%s\n' 'rustdesk-tailscale: PipeWire socket is not available; RustDesk Wayland screen capture will fail' >&2
fi

if [[ ! -e /dev/uinput ]]; then
  printf '%s\n' 'rustdesk-tailscale: /dev/uinput is missing; RustDesk Wayland keyboard/mouse fallback will not work' >&2
fi
HOST_BASH
	)"
}

if ! host_has_command rustdesk; then
	printf '%s\n' 'rustdesk-tailscale: rustdesk is not installed; add it to the Spectrum image' >&2
	exit 0
fi

install_rustdesk_apparmor_profile
install_rustdesk_selinux_policy
prepare_rustdesk_wayland_session
remove_rustdesk_flatpak
run_host_user_bash_file "$script_dir/rustdesk-desktop-entry.host.sh"
run_host_user_bash_file "$script_dir/rustdesk-merge-options.host.sh"
if run_host bash -c 'rpm -q rustdesk >/dev/null 2>&1'; then
	run_host_bash_file "$script_dir/rustdesk-merge-options.host.sh"
	run_host systemctl restart rustdesk.service
fi

if host_has_command tailscale; then
	run_host systemctl enable --now tailscaled 2>/dev/null || true
	if ! run_host_user tailscale status >/dev/null 2>&1; then
		printf '%s\n' 'rustdesk-tailscale: tailscale is installed but not authenticated; run tailscale up on this host' >&2
	fi
else
	printf '%s\n' 'rustdesk-tailscale: tailscale is not installed; install the tailscale host tool before relying on direct IP access' >&2
fi
