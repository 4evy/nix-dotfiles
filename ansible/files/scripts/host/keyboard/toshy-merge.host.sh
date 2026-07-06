#!/usr/bin/env bash
# shellcheck shell=bash

config_path=${1:?Toshy config path is required}
slice_merger=${2:?Toshy slice merger is required}
slice_dir=${3:?Toshy slice directory is required}
dropin_source=${4:?systemd drop-in source is required}
path_unit_source=${5:?systemd path unit source is required}
refresh_unit_source=${6:?systemd refresh service unit source is required}

if [[ ! -f $config_path ]]; then
	printf "%s\n" "toshy-kanata-chain: Toshy config was not found at $config_path" >&2
	exit 1
fi
if [[ ! -f $slice_merger ]]; then
	printf "%s\n" "toshy-kanata-chain: Toshy slice merger was not found at $slice_merger" >&2
	exit 1
fi
if [[ ! -d $slice_dir ]]; then
	printf "%s\n" "toshy-kanata-chain: Toshy slice directory was not found at $slice_dir" >&2
	exit 1
fi
if [[ ! -f $dropin_source ]]; then
	printf "%s\n" "toshy-kanata-chain: Toshy Kanata drop-in was not found at $dropin_source" >&2
	exit 1
fi
if [[ ! -f $path_unit_source ]]; then
	printf "%s\n" "toshy-kanata-chain: Toshy Kanata path unit was not found at $path_unit_source" >&2
	exit 1
fi
if [[ ! -f $refresh_unit_source ]]; then
	printf "%s\n" "toshy-kanata-chain: Toshy Kanata refresh unit was not found at $refresh_unit_source" >&2
	exit 1
fi

config_dir=${config_path%/*}
service_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
service_dropin_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/toshy-config.service.d"

ensure_dirs "$config_dir" "$service_dir" "$service_dropin_dir"
python3 "$slice_merger" "$config_path" "$slice_dir"
python3 -m py_compile "$config_path"
install_file_if_changed "$dropin_source" "$service_dropin_dir/10-dotfiles.conf"
install_file_if_changed "$path_unit_source" "$service_dir/toshy-kanata-device.path"
install_file_if_changed "$refresh_unit_source" "$service_dir/toshy-kanata-device.service"
systemctl --user daemon-reload || true
systemctl --user enable --now toshy-kanata-device.path || true
systemctl --user start toshy-kanata-device.service || true
