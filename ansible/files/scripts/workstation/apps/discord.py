from __future__ import annotations

import json
import os
import shlex
import sys
from pathlib import Path

from packaging.version import InvalidVersion, Version

from workstation.errors import DotfilesError
from workstation.lib.commands import run
from workstation.lib.files import ensure_directory, write_if_changed
from workstation.lib.host import user_config_home

DISCORD_FLAGS = (
    "--ozone-platform-hint=auto",
    "--disable-gpu-process-crash-limit",
    "--enable-gpu-rasterization",
)


def _flags_from_file(path: Path) -> list[str]:
    if not path.is_file():
        return []
    result: list[str] = []
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped and not stripped.startswith("#"):
            result.extend(shlex.split(stripped))
    return result


def _configure_gpu(discord_dir: Path) -> None:
    settings = discord_dir / "settings.json"
    try:
        value = json.loads(settings.read_text()) if settings.is_file() else {}
    except json.JSONDecodeError as error:
        raise DotfilesError(f"invalid Discord settings JSON: {settings}") from error
    value["enableHardwareAcceleration"] = True
    value["DANGEROUS_ENABLE_DEVTOOLS_ONLY_ENABLE_IF_YOU_KNOW_WHAT_YOURE_DOING"] = True
    switches = value.setdefault("chromiumSwitches", {})
    if not isinstance(switches, dict):
        switches = {}
        value["chromiumSwitches"] = switches
    switches["force_high_performance_gpu"] = True
    write_if_changed(settings, json.dumps(value, indent=2) + "\n", "0600")


def _patch_location(location: Path, equilotl: Path) -> None:
    if not equilotl.is_file() or not os.access(equilotl, os.X_OK):
        return
    asar = location / "resources/app.asar"
    build_info = location / "resources/build_info.json"
    if not asar.is_file() and not build_info.is_file():
        return
    if asar.is_file() and asar.stat().st_size <= 131072:
        content = asar.read_bytes()
        if b'"name": "discord"' in content and b"require(" in content:
            return
    run((equilotl, "--repair", "--location", location), check=False)


def _version_key(path: Path) -> tuple[int, Version | str]:
    name = path.name.removeprefix("app-")
    try:
        return 1, Version(name)
    except InvalidVersion:
        return 0, name


def _patch_current(discord_dir: Path, equilotl: Path) -> None:
    host = discord_dir / "Discord"
    if host.exists():
        try:
            target = host.resolve(strict=True)
        except OSError:
            pass
        else:
            _patch_location(target.parent, equilotl)
            return
    config_home = user_config_home()
    locations: set[Path] = set()
    for root in (discord_dir, config_home / "Discord"):
        if not root.is_dir():
            continue
        for marker in ("app-*/resources/app.asar", "app-*/resources/build_info.json"):
            locations.update(path.parent.parent for path in root.glob(marker))
    if locations:
        _patch_location(max(locations, key=_version_key), equilotl)


def main(argv: list[str] | None = None) -> int:
    arguments = list(sys.argv[1:] if argv is None else argv)
    config_home = user_config_home()
    discord_dir = config_home / "discord"
    discord_host = discord_dir / "Discord"
    package_bin = Path(sys.argv[0]).resolve().parent
    equilotl = Path(
        os.environ.get("DISCORD_EQUICORD_EQUILOTL", package_bin / "EquilotlCli-linux")
    )
    if arguments[:1] == ["--repair-only"]:
        _configure_gpu(discord_dir)
        _patch_current(discord_dir, equilotl)
        return 0
    if not discord_host.is_file() or not os.access(discord_host, os.X_OK):
        ensure_directory(discord_dir)
        bootstrap = next(
            (
                path
                for path in (
                    Path("/usr/share/discord/updater_bootstrap"),
                    Path("/opt/discord/updater_bootstrap"),
                    Path("/opt/Discord/updater_bootstrap"),
                )
                if path.is_file() and os.access(path, os.X_OK)
            ),
            None,
        )
        if bootstrap is None:
            raise DotfilesError(
                "discord-equicord: Discord updater bootstrap was not found"
            )
        zenity = "--no-zenity" if sys.stdout.isatty() else "--zenity"
        run((bootstrap, zenity, discord_dir, "stable", "https://updates.discord.com/"))
    flags = [*DISCORD_FLAGS, *_flags_from_file(config_home / "discord-flags.conf")]
    _configure_gpu(discord_dir)
    _patch_current(discord_dir, equilotl)
    result = run((discord_host, *flags, *arguments), check=False)
    _patch_current(discord_dir, equilotl)
    return result.returncode


def entrypoint() -> None:
    try:
        raise SystemExit(main())
    except DotfilesError as error:
        raise SystemExit(str(error)) from error
