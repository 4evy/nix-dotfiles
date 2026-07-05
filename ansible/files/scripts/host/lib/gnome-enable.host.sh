#!/usr/bin/env bash
# shellcheck shell=bash

if ! gsettings_available; then
	exit 0
fi

for uuid in "$@"; do
	current=$(gsettings get org.gnome.shell enabled-extensions 2>/dev/null || printf "[]")
	single="'$uuid'"
	double="\"$uuid\""
	case "$current" in
		*"$single"* | *"$double"*)
			continue
			;;
		"@as []" | "[]")
			next="[$single]"
			;;
		\[*\])
			inner=${current#"["}
			inner=${inner%"]"}
			if [[ -z ${inner//[[:space:]]/} ]]; then
				next="[$single]"
			else
				next="[$inner, $single]"
			fi
			;;
		*)
			continue
			;;
	esac
	gsettings set org.gnome.shell enabled-extensions "$next"
done
