#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

if ! command -v tailscale >/dev/null 2>&1 || ! command -v tailscaled >/dev/null 2>&1; then
	printf "%s\n" "tailscale-bluefin: Tailscale is not installed; add it to the Spectrum image and switch to the rebuilt image" >&2
	exit 0
fi

systemctl enable tailscaled
systemctl start tailscaled || true
