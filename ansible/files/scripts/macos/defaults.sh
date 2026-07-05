#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh

require_arg_count 0 0 "$@"
require_command defaults killall

write_default() {
	local domain=${1:?defaults domain is required}
	local key=${2:?defaults key is required}
	local value_type=${3:?defaults value type is required}
	local value=${4:?defaults value is required}

	case $value_type in
		bool | int | string) ;;
		*) die "unsupported defaults value type for $domain $key: $value_type" ;;
	esac

	defaults write "$domain" "$key" "-$value_type" "$value"
}

write_domain_defaults() {
	local domain=${1:?defaults domain is required}
	shift

	if (($# % 3 != 0)); then
		die "defaults for $domain must be key/type/value triples"
	fi

	while (($# > 0)); do
		write_default "$domain" "$1" "$2" "$3"
		shift 3
	done
}

restart_apps() {
	local app

	for app; do
		killall "$app" >/dev/null 2>&1 || true
	done
}

write_default com.apple.LaunchServices LSQuarantine bool false
write_default com.apple.SoftwareUpdate AutomaticallyInstallMacOSUpdates bool false

write_domain_defaults NSGlobalDomain \
	KeyRepeat int 2 \
	AppleMetricUnits bool true \
	AppleInterfaceStyleSwitchesAutomatically bool true \
	NSAutomaticWindowAnimationsEnabled bool false \
	NSDocumentSaveNewDocumentsToCloud bool false \
	NSAutomaticCapitalizationEnabled bool false \
	NSAutomaticSpellingCorrectionEnabled bool false \
	NSAutomaticPeriodSubstitutionEnabled bool false \
	NSAutomaticDashSubstitutionEnabled bool false \
	NSAutomaticQuoteSubstitutionEnabled bool false

write_domain_defaults com.apple.menuextra.clock \
	Show24Hour bool true \
	ShowSeconds bool true \
	ShowDate int 2

write_domain_defaults com.apple.finder \
	AppleShowAllFiles bool false \
	AppleShowAllExtensions bool true \
	FXEnableExtensionChangeWarning bool false \
	FXPreferredViewStyle string icnv \
	QuitMenuItem bool true \
	ShowPathbar bool true

write_domain_defaults com.apple.dock \
	autohide bool true \
	magnification bool false \
	orientation string bottom \
	show-recents bool false \
	showhidden bool true \
	tilesize int 65 \
	wvous-tl-corner int 1 \
	wvous-tr-corner int 1 \
	wvous-bl-corner int 1 \
	wvous-br-corner int 1

restart_apps Dock Finder SystemUIServer
