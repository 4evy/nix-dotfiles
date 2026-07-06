#!/usr/bin/env bash
# shellcheck shell=bash

if ! command -v kmscon >/dev/null 2>&1; then
	printf '%s\n' 'kmscon: /usr/bin/kmscon is not installed; rebuild and boot into the Spectrum image first'
	exit 0
fi

repo_root=${1:?repo root is required}
host_script_root=${2:?host script root is required}
python=/usr/bin/python3

require_file /usr/lib/systemd/system/kmsconvt@.service
require_file "$python"
require_file "$repo_root/dotfiles/.chezmoitemplates/catppuccin_palette.json"
require_file "$host_script_root/desktop/kmscon-theme-config.py"
require_command infocmp ln mv rm systemctl

infocmp kmscon >/dev/null || die "kmscon terminfo is not installed"
[[ -x $python ]] || die "python is not executable: $python"

ensure_dir_mode 0755 /etc/default /etc/kmscon /etc/systemd/system /etc/systemd/system/kmsconvt@.service.d /usr/local/libexec/dotfiles /usr/local/share/dotfiles
install_file_if_changed "$host_script_root/desktop/kmscon-theme-config.py" /usr/local/libexec/dotfiles/kmscon-theme-config 0755
install_file_if_changed "$repo_root/dotfiles/.chezmoitemplates/catppuccin_palette.json" /usr/local/share/dotfiles/catppuccin_palette.json 0644
rm -f -- /usr/local/libexec/dotfiles/uv

if [[ ! -e /etc/default/kmscon-dotfiles ]]; then
	cat <<'EOF' | write_stdin_if_changed /etc/default/kmscon-dotfiles 0644
# Sofia center is the default location.
# DOTFILES_KMSCON_LATITUDE=
# DOTFILES_KMSCON_LONGITUDE=
# DOTFILES_KMSCON_THEME=latte
# DOTFILES_KMSCON_THEME=frappe
EOF
fi

set -a
# shellcheck disable=SC1091
[[ ! -f /etc/default/kmscon-dotfiles ]] || source /etc/default/kmscon-dotfiles
set +a
"$python" /usr/local/libexec/dotfiles/kmscon-theme-config /usr/local/share/dotfiles/catppuccin_palette.json /etc/kmscon/kmscon.conf

cat <<EOF | write_stdin_if_changed /etc/systemd/system/kmsconvt@.service.d/10-dotfiles-theme.conf 0644
[Service]
EnvironmentFile=-/etc/default/kmscon-dotfiles
ExecStartPre=-$python /usr/local/libexec/dotfiles/kmscon-theme-config /usr/local/share/dotfiles/catppuccin_palette.json /etc/kmscon/kmscon.conf
EOF

cat <<'EOF' | write_stdin_if_changed /etc/systemd/system/kmscon-theme-refresh.service 0644
[Unit]
Description=Refresh KMSCON theme
Documentation=https://github.com/kmscon/kmscon

[Service]
Type=oneshot
EnvironmentFile=-/etc/default/kmscon-dotfiles
ExecStart=-/usr/bin/python3 /usr/local/libexec/dotfiles/kmscon-theme-config /usr/local/share/dotfiles/catppuccin_palette.json /etc/kmscon/kmscon.conf
ExecStartPost=/usr/bin/bash -c 'for unit in kmsconvt@tty{1..6}.service; do tty=${unit#*@}; tty=${tty%.service}; if ! /usr/bin/loginctl list-sessions --no-legend | /usr/bin/awk -v tty="$tty" "$5 == tty { found=1 } END { exit found ? 0 : 1 }"; then /usr/bin/systemctl try-restart "$unit"; fi; done'
EOF

cat <<'EOF' | write_stdin_if_changed /etc/systemd/system/kmscon-theme-refresh.timer 0644
[Unit]
Description=Refresh KMSCON theme around day/night changes

[Timer]
OnBootSec=2m
OnCalendar=*-*-* 00,02,04,06,18,20,22:05:00
Persistent=true

[Install]
WantedBy=timers.target
EOF

autovt_link=/etc/systemd/system/autovt@.service
if [[ -e $autovt_link && ! -L $autovt_link ]]; then
	mv -f -- "$autovt_link" "${autovt_link}.dotfiles-disabled"
fi
ln -sfn /usr/lib/systemd/system/kmsconvt@.service "$autovt_link"
rm -f -- /etc/systemd/system/getty.target.wants/getty@tty1.service
ln -sfn /usr/lib/systemd/system/kmsconvt@.service /etc/systemd/system/getty.target.wants/kmsconvt@tty1.service
systemctl daemon-reload
systemctl enable --now kmscon-theme-refresh.timer >/dev/null

printf '%s\n' 'kmscon: configured autovt@.service to use KMSCON for text virtual terminals; reboot to use it'
