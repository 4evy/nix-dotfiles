#!/usr/bin/env python3

import os
import subprocess
import tarfile
from pathlib import Path


def main() -> int:
    zig_arch = os.environ["ZIG_ARCH"]
    zig_version = os.environ["ZIG_VERSION"]
    subprocess.run(
        (
            "dnf",
            "-y",
            "install",
            "--setopt=install_weak_deps=False",
            "ca-certificates",
            "curl",
            "file",
            "findutils",
            "gcc",
            "gcc-c++",
            "gettext",
            "glibc-devel",
            "gtk4-devel",
            "gtk4-layer-shell-devel",
            "libadwaita-devel",
            "libxml2",
            "pkgconf-pkg-config",
            "tar",
            "xz",
        ),
        check=True,
    )
    name = f"zig-{zig_arch}-{zig_version}"
    archive = Path(f"{name}.tar.xz")
    subprocess.run(
        (
            "curl",
            "-fsSLo",
            archive,
            f"https://ziglang.org/download/{zig_version}/{name}.tar.xz",
        ),
        check=True,
    )
    with tarfile.open(archive) as source:
        source.extractall("/opt", filter="data")
    environment = dict(os.environ)
    environment["PATH"] = f"/opt/{name}:{environment['PATH']}"
    subprocess.run(
        ("zig", "build", "-p", "/work/stage", "-Doptimize=ReleaseFast"),
        check=True,
        env=environment,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
