#!/usr/bin/env bash
# shellcheck shell=bash

version=$(gnome-shell --version)
if [[ $version =~ ([0-9]+) ]]; then
	printf "%s\n" "${BASH_REMATCH[1]}"
fi
