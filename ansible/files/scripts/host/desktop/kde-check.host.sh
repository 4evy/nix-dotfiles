#!/usr/bin/env bash
# shellcheck shell=bash

command -v kwin_wayland >/dev/null 2>&1 ||
	command -v plasmashell >/dev/null 2>&1 ||
	test -d /usr/share/plasma ||
	test -f /usr/share/wayland-sessions/plasma.desktop
