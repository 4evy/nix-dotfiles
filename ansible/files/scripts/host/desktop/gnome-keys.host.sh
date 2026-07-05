#!/usr/bin/env bash
# shellcheck shell=bash

schema_dir=${1:?schema dir is required}

if ! gsettings_available; then
	exit 0
fi
if [[ ! -d $schema_dir ]]; then
	exit 0
fi

GSETTINGS_SCHEMA_DIR=$schema_dir gsettings set org.gnome.shell.extensions.hyper-window-tiling move-up "['<Super><Control><Alt><Shift>w']"
GSETTINGS_SCHEMA_DIR=$schema_dir gsettings set org.gnome.shell.extensions.hyper-window-tiling move-left "['<Super><Control><Alt><Shift>a']"
GSETTINGS_SCHEMA_DIR=$schema_dir gsettings set org.gnome.shell.extensions.hyper-window-tiling move-down "['<Super><Control><Alt><Shift>s']"
GSETTINGS_SCHEMA_DIR=$schema_dir gsettings set org.gnome.shell.extensions.hyper-window-tiling move-right "['<Super><Control><Alt><Shift>d']"
GSETTINGS_SCHEMA_DIR=$schema_dir gsettings set org.gnome.shell.extensions.hyper-window-tiling move-max-almost "['<Super><Control><Alt><Shift>Return']"
GSETTINGS_SCHEMA_DIR=$schema_dir gsettings set org.gnome.shell.extensions.hyper-window-tiling move-max "['<Super><Control><Alt><Shift>backslash']"
gsettings set org.gnome.desktop.wm.keybindings switch-to-workspace-left "[]"
gsettings set org.gnome.desktop.wm.keybindings switch-to-workspace-right "[]"
gsettings set org.gnome.desktop.wm.keybindings move-to-workspace-left "[]"
gsettings set org.gnome.desktop.wm.keybindings move-to-workspace-right "[]"
