#!/usr/bin/env bash
# shellcheck shell=bash

host_user=${1:?host user is required}
kanata_config=${2:?kanata config path is required}
uinput_rules=${3:?uinput rules path is required}
kanata_unit=${4:?kanata unit path is required}

ensure_group() {
	local group_name=${1:?group name is required}
	local group_entry

	if command -v getent >/dev/null 2>&1 && getent group "$group_name" >/dev/null 2>&1; then
		return
	fi
	if [[ -f /usr/lib/group ]]; then
		group_entry=$(awk -F: -v group="$group_name" '$1 == group { print; exit }' /usr/lib/group)
	fi
	if [[ -n ${group_entry:-} ]]; then
		printf '%s\n' "$group_entry" >>/etc/group
		return
	fi
	groupadd --system "$group_name"
}

ensure_dir_mode 0755 /etc/kanata /etc/modules-load.d /etc/udev/rules.d /etc/systemd/system
install_file_if_changed "$kanata_config" /etc/kanata/kanata.kbd

ensure_group input
ensure_group uinput
usermod -aG input,uinput "$host_user"

/usr/local/bin/kanata --cfg "$kanata_config" --check --no-wait

printf "%s\n" uinput | write_stdin_if_changed /etc/modules-load.d/uinput.conf 0644
install_file_if_changed "$uinput_rules" /etc/udev/rules.d/70-dotfiles-uinput.rules
install_file_if_changed "$kanata_unit" /etc/systemd/system/kanata-main.service

modprobe uinput || true
reload_udev_rules
trigger_udev --subsystem-match=misc --attr-match=name=uinput
systemctl daemon-reload
if [[ ${DOTFILES_KEEP_INPUT_REMAPPER:-0} != 1 ]] && systemctl cat input-remapper.service >/dev/null 2>&1; then
	systemctl disable --now input-remapper.service || true
	systemctl mask input-remapper.service || true
	systemctl daemon-reload
fi
if systemctl cat kanata.service >/dev/null 2>&1; then
	systemctl disable --now kanata.service || true
	if [[ -e /etc/systemd/system/kanata.service ]] && ! [[ -L /etc/systemd/system/kanata.service ]]; then
		mv -f -- /etc/systemd/system/kanata.service /etc/systemd/system/kanata.service.dotfiles-disabled
	fi
	systemctl mask kanata.service || true
	systemctl daemon-reload
fi
systemctl enable --now kanata-main.service
systemctl restart kanata-main.service
