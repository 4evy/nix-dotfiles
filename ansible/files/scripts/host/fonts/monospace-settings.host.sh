#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

if gsettings_available; then
	gsettings_set_if_writable org.gnome.desktop.interface font-name "Noto Sans 11"
	gsettings_set_if_writable org.gnome.desktop.interface document-font-name "Noto Sans 12"
	gsettings_set_if_writable org.gnome.desktop.interface monospace-font-name "JetBrainsMono Nerd Font Mono 11"
fi
