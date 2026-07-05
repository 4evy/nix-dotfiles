#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

zig_arch=${ZIG_ARCH:?ZIG_ARCH is required}
zig_version=${ZIG_VERSION:?ZIG_VERSION is required}

dnf -y install --setopt=install_weak_deps=False \
	ca-certificates \
	curl \
	file \
	findutils \
	gcc \
	gcc-c++ \
	gettext \
	glibc-devel \
	gtk4-devel \
	gtk4-layer-shell-devel \
	libadwaita-devel \
	libxml2 \
	pkgconf-pkg-config \
	tar \
	xz

zig_name="zig-${zig_arch}-${zig_version}"
curl -fsSLO "https://ziglang.org/download/${zig_version}/${zig_name}.tar.xz"
tar -xf "${zig_name}.tar.xz" -C /opt
export PATH="/opt/${zig_name}:${PATH}"

zig build -p /work/stage -Doptimize=ReleaseFast
