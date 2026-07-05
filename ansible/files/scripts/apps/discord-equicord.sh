#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
for host_lib_dir in \
	"$script_dir/../lib/dotfiles/host/lib" \
	"$script_dir/../host/lib"; do
	if [[ -r $host_lib_dir/entrypoint.sh ]]; then
		source -p "$host_lib_dir" entrypoint.sh
		break
	fi
done
if [[ ${DOTFILES_HOST_LIB_ENTRYPOINT_SOURCED:-0} != 1 ]]; then
	printf 'discord-equicord: failed to locate dotfiles host script library\n' >&2
	exit 1
fi
source_host_lib fs

channel=stable
download_url=https://updates.discord.com/
config_home=${XDG_CONFIG_HOME:-"$HOME/.config"}
discord_dir="$config_home/discord"
discord_host="$discord_dir/Discord"
equilotl=${DISCORD_EQUICORD_EQUILOTL:-"$script_dir/EquilotlCli-linux"}
discord_flags=(
	--ozone-platform-hint=auto
	--disable-gpu-process-crash-limit
	--enable-gpu-rasterization
)

append_flags_file() {
	local file=${1:?flags file is required}
	[[ -r "$file" ]] || return 0

	local parsed_flags
	local -a file_flags=()

	if ! parsed_flags=$(sed '/^[[:space:]]*#/d;/^[[:space:]]*$/d' "$file" | xargs -r printf '%s\n'); then
		printf 'discord-equicord: failed to parse flags file: %s\n' "$file" >&2
		return 1
	fi

	[[ -n $parsed_flags ]] || return 0
	mapfile -t file_flags <<<"$parsed_flags"
	discord_flags+=("${file_flags[@]}")
}

configure_gpu_settings() {
	local settings_path="$discord_dir/settings.json"

	command -v jq >/dev/null || return 0
	if [[ -f $settings_path ]]; then
		jq '
      .enableHardwareAcceleration = true |
      .DANGEROUS_ENABLE_DEVTOOLS_ONLY_ENABLE_IF_YOU_KNOW_WHAT_YOURE_DOING = true |
      .chromiumSwitches = ((.chromiumSwitches // {}) + {
        force_high_performance_gpu: true
      })
    ' "$settings_path" | write_stdin_if_changed "$settings_path" 0600
	else
		jq -n '{
      enableHardwareAcceleration: true,
      DANGEROUS_ENABLE_DEVTOOLS_ONLY_ENABLE_IF_YOU_KNOW_WHAT_YOURE_DOING: true,
      chromiumSwitches: {
        force_high_performance_gpu: true
      }
    }' | write_stdin_if_changed "$settings_path" 0600
	fi
}

find_bootstrap() {
	local candidate

	for candidate in /usr/share/discord/updater_bootstrap /opt/discord/updater_bootstrap /opt/Discord/updater_bootstrap; do
		if [[ -x $candidate ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done

	return 1
}

patch_location() {
	local location=${1:?location is required}
	local app_asar="$location/resources/app.asar"
	local size

	if [[ ! -x $equilotl ]]; then
		return 0
	fi

	if [[ ! -f $app_asar && ! -f "$location/resources/build_info.json" ]]; then
		return 0
	fi

	if [[ -f $app_asar ]]; then
		size=$(wc -c <"$app_asar" 2>/dev/null || printf '%s\n' 999999)
		if [[ $size -le 131072 ]] &&
			grep -aq '"name": "discord"' "$app_asar" &&
			grep -aq 'require(' "$app_asar"; then
			return 0
		fi
	fi

	"$equilotl" --repair --location "$location" || true
}

patch_current_discord() {
	local location target

	if [[ -e $discord_host ]]; then
		target=$(readlink -f "$discord_host" 2>/dev/null || true)
		if [[ -n $target ]]; then
			patch_location "$(dirname "$target")"
			return 0
		fi
	fi

	if IFS= read -r -d '' location < <(
		find \
			"$discord_dir" \
			"$config_home/Discord" \
			-maxdepth 3 -type f \
			\( -path '*/app-*/resources/app.asar' -o -path '*/app-*/resources/build_info.json' \) \
			-printf '%h\0' 2>/dev/null |
			sed -z 's|/resources$||' |
			sort -zVru
	); then
		patch_location "$location"
	fi
}

if [[ ${1:-} == --repair-only ]]; then
	configure_gpu_settings
	patch_current_discord
	exit 0
fi

if [[ ! -x $discord_host ]]; then
	ensure_dir "$discord_dir"
	bootstrap=$(find_bootstrap)

	if [[ -t 1 ]]; then
		zenity=--no-zenity
	else
		zenity=--zenity
	fi

	"$bootstrap" "$zenity" "$discord_dir" "$channel" "$download_url" >/dev/null
fi

append_flags_file "$config_home/discord-flags.conf"
configure_gpu_settings

patch_current_discord
"$discord_host" "${discord_flags[@]}" "$@"
status=$?
patch_current_discord
exit "$status"
