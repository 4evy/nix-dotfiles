#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

installation=${1:?installation is required}

active_nvidia_gl_drivers() {
	flatpak --gl-drivers 2>/dev/null | awk "/^nvidia-/ { print }"
}

installation_has_flathub() {
	local installation=${1:?installation is required}

	flatpak remotes "--${installation}" --columns=name 2>/dev/null | grep -Fxq flathub
}

runtime_installed() {
	local installation=${1:?installation is required}
	local runtime_ref=${2:?runtime ref is required}

	flatpak info "--${installation}" "$runtime_ref" >/dev/null 2>&1
}

install_nvidia_gl_runtime() {
	local driver=${1:?driver is required}
	local remote=flathub
	local runtime="org.freedesktop.Platform.GL.${driver}//1.4"

	if ! installation_has_flathub "$installation"; then
		printf "%s\n" "flatpak-nvidia: ${installation} installation has no flathub remote; skipping"
		return 0
	fi

	if runtime_installed "$installation" "$runtime"; then
		printf "%s\n" "flatpak-nvidia: ${runtime} already installed in ${installation} installation"
		return 0
	fi

	printf "%s\n" "flatpak-nvidia: ensuring ${runtime} in ${installation} installation"
	flatpak install "--${installation}" --noninteractive "$remote" "$runtime"
}

install_nvidia_vaapi_runtime() {
	local remote=flathub
	local runtime=org.freedesktop.Platform.VAAPI.nvidia
	local app branch

	if ! installation_has_flathub "$installation"; then
		return 0
	fi

	flatpak remote-ls "--${installation}" "$remote" --runtime --columns=application,branch 2>/dev/null |
		while read -r app branch; do
			[[ $app == "$runtime" && -n ${branch:-} ]] || continue
			if runtime_installed "$installation" "${runtime}//${branch}"; then
				printf "%s\n" "flatpak-nvidia: ${runtime}//${branch} already installed in ${installation} installation"
				continue
			fi
			printf "%s\n" "flatpak-nvidia: ensuring ${runtime}//${branch} in ${installation} installation"
			flatpak install "--${installation}" --noninteractive "$remote" "${runtime}//${branch}"
		done
}

mapfile -t drivers < <(active_nvidia_gl_drivers)
if ((${#drivers[@]} == 0)); then
	printf "%s\n" "flatpak-nvidia: no active NVIDIA Flatpak GL driver; skipping"
	exit 0
fi

for driver in "${drivers[@]}"; do
	install_nvidia_gl_runtime "$driver"
done

install_nvidia_vaapi_runtime
