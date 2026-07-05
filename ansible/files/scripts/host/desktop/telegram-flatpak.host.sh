#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

app_id=org.telegram.desktop

if ! flatpak info "$app_id" >/dev/null 2>&1; then
	printf '%s\n' "telegram-flatpak: ${app_id} is not installed; skipping"
	exit 0
fi

# Telegram/Qt can hit EGL context creation failures on native Wayland with
# NVIDIA. Run only Telegram through XWayland while keeping hardware GL enabled.
flatpak override --user --nosocket=wayland --socket=x11 --env=QT_QPA_PLATFORM=xcb "$app_id"
flatpak kill "$app_id" >/dev/null 2>&1 || true

printf '%s\n' "telegram-flatpak: configured ${app_id} to use XWayland"
