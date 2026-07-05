#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../lib" entrypoint.sh
source_host_lib fs
source_host_lib host
source_host_lib hyper_window_tiling

require_arg_count 0 0 "$@"

if ! run_host_user_bash_file "$script_dir/kde-check.host.sh"; then
	printf '%s\n' "hyper-window-tiling: KDE Plasma is not available; skipping"
	exit 0
fi

require_command install

repo_dir=$DOTFILES_REPO_ROOT
build_root=$(hyper_window_tiling_build "$repo_dir/dotfiles")
kde_source="$build_root/kde/hyper-window-tiling"
require_file "$kde_source/metadata.json"
require_file "$kde_source/contents/code/main.js"

state_dir=${XDG_STATE_HOME:-$HOME/.local/state}/dotfiles/hyper-window-tiling
install_source="$state_dir/kwin-script/hyper-window-tiling"

fresh_dir "$install_source"
install_file_if_changed "$kde_source/metadata.json" "$install_source/metadata.json"
install_file_if_changed "$kde_source/contents/code/main.js" "$install_source/contents/code/main.js"

installed_with_kpackage=false
kpackage=$(run_host_user_bash 'command -v kpackagetool6 || command -v kpackagetool5 || true')
if [[ -n $kpackage ]]; then
	if run_host_user "$kpackage" --type KWin/Script --upgrade "$install_source" >/dev/null 2>&1 ||
		run_host_user "$kpackage" --type KWin/Script --install "$install_source" >/dev/null 2>&1; then
		installed_with_kpackage=true
	fi
fi

if [[ $installed_with_kpackage != true ]]; then
	host_data_home=$(run_host_user_bash "printf \"%s\n\" \"\${XDG_DATA_HOME:-\$HOME/.local/share}\"")
	for destination in \
		"$host_data_home/kwin/scripts/hyper-window-tiling" \
		"$host_data_home/kwin-wayland/scripts/hyper-window-tiling"; do
		replace_dir "$install_source" "$destination"
	done
fi

kwriteconfig=$(run_host_user_bash 'command -v kwriteconfig6 || command -v kwriteconfig5 || true')
if [[ -n $kwriteconfig ]]; then
	run_host_user \
		"$kwriteconfig" \
		--file kwinrc \
		--group Plugins \
		--key hyper-window-tilingEnabled \
		true >/dev/null 2>&1 || true
fi

qdbus=$(run_host_user_bash 'command -v qdbus6 || command -v qdbus || true')
if [[ -n $qdbus ]]; then
	run_host_user "$qdbus" org.kde.KWin /KWin reconfigure >/dev/null 2>&1 || true
fi

printf '%s\n' "hyper-window-tiling: installed KDE script"
