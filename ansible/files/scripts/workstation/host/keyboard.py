from __future__ import annotations

import os
import re
import shutil
import sys
import tempfile
from pathlib import Path

import typer

from workstation.automation import automation_check_mode, current_context
from workstation.automation_models import OperationResult
from workstation.console import console, error_console
from workstation.errors import DotfilesError
from workstation.lib.commands import output, require_commands, run, which
from workstation.lib.files import (
    ensure_directory,
    fresh_directory,
    install_file_if_changed,
    require_directory,
    require_file,
    write_if_changed,
)
from workstation.lib.host import (
    enable_gnome_extensions,
    gsettings_available,
    user_cache_home,
    user_config_home,
    user_data_home,
    user_state_home,
)
from workstation.lib.http import download, get
from workstation.lib.paths import find_repo_root

app = typer.Typer(no_args_is_help=True, pretty_exceptions_enable=False)


def _source_root() -> Path:
    return find_repo_root(Path(__file__)) / "ansible/files/scripts/host/keyboard"


def _kanata_staging_dir() -> Path:
    context = current_context()
    data_dir = context.data_dir if context is not None else None
    return (data_dir or user_data_home() / "dotfiles") / "host/bin-staging/kanata"


@app.command("kanata-build")
def build_kanata() -> OperationResult:
    """Build a staged Kanata binary with command support."""
    executable = _kanata_staging_dir() / "root/bin/kanata"
    if automation_check_mode():
        return OperationResult(
            changed=True,
            msg="Would build a staged Kanata binary",
            data={"executable": str(executable)},
        )
    require_commands("cargo", "git")
    staging = fresh_directory(_kanata_staging_dir())
    install_root = staging / "root"
    run((
        "cargo",
        "install",
        "--git",
        "https://github.com/jtroo/kanata",
        "kanata",
        "--features",
        "cmd",
        "--root",
        install_root,
        "--force",
        "--locked",
    ))
    console.print(executable)
    return OperationResult(
        changed=True,
        msg="Built a staged Kanata binary",
        data={"executable": str(executable)},
    )


GNOME_EXTENSIONS = {
    "focused-window-dbus@flexagoon.com": "Focused Window D-Bus",
    "window-calls-extended@hseliger.eu": "Window Calls Extended",
    "xremap@k0kubun.com": "Xremap",
    "appindicatorsupport@rgcjonas.gmail.com": "AppIndicator and KStatusNotifierItem Support",
}


def _gnome_shell_major() -> str:
    version = output(("gnome-shell", "--version"))
    match = re.search(r"\d+", version)
    if not match:
        raise DotfilesError(
            "toshy-gnome-window-context: failed to detect host GNOME Shell version"
        )
    return match.group()


@app.command("toshy-gnome-context")
def toshy_gnome_context() -> OperationResult:
    """Install compatible GNOME window-context extensions for Toshy."""
    if which("gnome-shell") is None or which("gnome-extensions") is None:
        console.print(
            "toshy-gnome-window-context: GNOME Shell is not available; skipping"
        )
        return OperationResult(msg="GNOME Shell is not available")
    if automation_check_mode():
        return OperationResult(
            changed=True, msg="Would reconcile Toshy GNOME context extensions"
        )
    shell_version = _gnome_shell_major()
    origin = "https://extensions.gnome.org"
    cache = ensure_directory(user_cache_home() / "dotfiles/gnome-extensions")
    installed_window_context = 0
    for uuid, label in GNOME_EXTENSIONS.items():
        response = get(
            f"{origin}/extension-info/",
            params={"uuid": uuid, "shell_version": shell_version},
        )
        metadata = response.json()
        download_url = metadata.get("download_url")
        version = metadata.get("version")
        if not download_url or version is None:
            console.print(
                f"toshy-gnome-window-context: {label} has no compatible "
                f"GNOME Shell {shell_version} build; skipping"
            )
            continue
        archive = cache / f"{uuid}-{version}.shell-extension.zip"
        download(f"{origin}{download_url}", archive)
        installed_uuid = output((
            "gnome-extensions",
            "install",
            "--force",
            "--print-uuid",
            archive,
        ))
        if installed_uuid != uuid:
            raise DotfilesError(
                "toshy-gnome-window-context: installed UUID "
                f"{installed_uuid} did not match {uuid}"
            )
        run(("gnome-extensions", "enable", uuid), check=False, capture=True)
        enable_gnome_extensions(uuid)
        if uuid != "appindicatorsupport@rgcjonas.gmail.com":
            installed_window_context += 1
        console.print(
            f"toshy-gnome-window-context: installed {label} for GNOME Shell {shell_version}"
        )
    if installed_window_context == 0:
        raise DotfilesError(
            "toshy-gnome-window-context: no Toshy-compatible GNOME window "
            f"context extension was available for GNOME Shell {shell_version}"
        )
    if gsettings_available():
        current = run(
            ("gsettings", "get", "org.gnome.mutter", "overlay-key"),
            check=False,
            capture=True,
        ).stdout.strip()
        if current == "''":
            run(("gsettings", "set", "org.gnome.mutter", "overlay-key", "Super_L"))
    return OperationResult(
        changed=True,
        msg="Reconciled Toshy GNOME context extensions",
        data={"shell_version": shell_version},
    )


def _has_toshy_slices(config: Path) -> bool:
    if not config.is_file():
        return False
    content = config.read_text(encoding="utf-8")
    return (
        "SLICE_MARK_START: keymapper_api" in content
        and "SLICE_MARK_START: barebones_user_cfg" in content
    )


def _set_no_display(desktop_file: Path) -> None:
    if not desktop_file.is_file():
        return
    content = desktop_file.read_text(encoding="utf-8")
    pattern = re.compile(r"^[ \t]*NoDisplay=.*$", re.MULTILINE)
    if pattern.search(content):
        updated = pattern.sub("NoDisplay=true", content)
    else:
        updated = content.rstrip("\n") + "\n\nNoDisplay=true\n"
    write_if_changed(desktop_file, updated)


def _hide_toshy_surfaces() -> None:
    for desktop_file in (
        user_data_home() / "applications/Toshy_Tray.desktop",
        user_data_home() / "applications/app.toshy.preferences.desktop",
        user_config_home() / "autostart/Toshy_Tray.desktop",
        user_config_home() / "autostart/app.toshy.preferences.desktop",
    ):
        _set_no_display(desktop_file)
    if which("systemctl") is not None:
        run(
            ("systemctl", "--user", "disable", "--now", "toshy-tray.service"),
            check=False,
            capture=True,
        )


def _merge_toshy(
    config: Path,
    merger: Path,
    slices: Path,
    source: Path,
) -> None:
    require_file(config)
    require_file(merger)
    require_directory(slices)
    config_dir = ensure_directory(config.parent)
    del config_dir
    service_dir = ensure_directory(user_config_home() / "systemd/user")
    dropin_dir = ensure_directory(service_dir / "toshy-config.service.d")
    run((sys.executable, merger, config, slices))
    run((sys.executable, "-m", "py_compile", config))
    install_file_if_changed(
        source / "toshy-kanata.conf", dropin_dir / "10-dotfiles.conf"
    )
    install_file_if_changed(
        source / "toshy-kanata-device.path", service_dir / "toshy-kanata-device.path"
    )
    install_file_if_changed(
        source / "toshy-kanata-device.service",
        service_dir / "toshy-kanata-device.service",
    )
    run(("systemctl", "--user", "daemon-reload"), check=False)
    run(
        ("systemctl", "--user", "enable", "--now", "toshy-kanata-device.path"),
        check=False,
    )
    run(
        ("systemctl", "--user", "start", "toshy-kanata-device.service"),
        check=False,
    )


@app.command("toshy-kanata-chain")
def toshy_kanata_chain() -> OperationResult:
    """Install Toshy and merge the Kanata-only dotfiles slices."""
    if automation_check_mode():
        return OperationResult(
            changed=True, msg="Would reconcile the Toshy and Kanata device chain"
        )
    require_commands("git")
    repository = find_repo_root(Path(__file__))
    source = _source_root()
    root = ensure_directory(user_state_home() / "dotfiles/toshy")
    toshy_repo = root / "Toshy"
    toshy_ref = os.environ.get("TOSHY_REF", "Toshy_v26.06.0")
    automation = require_file(source / "toshy-setup.py")
    merger = require_file(repository / "packages/toshy/merge-slices.py")
    slices = require_directory(repository / "packages/toshy/slices")
    for name in ("keymapper_api.py", "kbtype_override.py", "barebones_user_cfg.py"):
        require_file(slices / name)
    config = user_config_home() / "toshy/toshy_config.py"

    if (toshy_repo / ".git").is_dir():
        run((
            "git",
            "-C",
            toshy_repo,
            "fetch",
            "--depth",
            "1",
            "--filter=blob:none",
            "origin",
            toshy_ref,
        ))
        run(("git", "-C", toshy_repo, "checkout", "--force", "FETCH_HEAD"))
    else:
        if toshy_repo.exists():
            shutil.rmtree(toshy_repo)
        run((
            "git",
            "clone",
            "--depth",
            "1",
            "--filter=blob:none",
            "--no-checkout",
            "--branch",
            toshy_ref,
            "https://github.com/RedBearAK/Toshy.git",
            toshy_repo,
        ))
        run(("git", "-C", toshy_repo, "checkout", "--force", toshy_ref))

    force_install = os.environ.get("TOSHY_RUN_INSTALLER") == "1"
    if not _has_toshy_slices(config) or force_install:
        console.print(
            "toshy-kanata-chain: launching upstream Toshy barebones installer "
            "with dotfiles automation."
        )
        with tempfile.TemporaryDirectory(prefix="toshy-automation-") as temporary:
            automation_dir = Path(temporary)
            arguments = ["install", "--barebones-config"]
            distro = os.environ.get("TOSHY_DISTRO_OVERRIDE")
            if distro:
                arguments.extend(("--override-distro", distro))
            elif Path("/run/ostree-booted").exists() or which("rpm-ostree") is not None:
                arguments.extend(("--override-distro", "silverblue"))
            if os.environ.get("TOSHY_SKIP_NATIVE") == "1":
                arguments.append("--skip-native")
            if os.environ.get("TOSHY_NO_DBUS_PYTHON") == "1":
                arguments.append("--no-dbus-python")
            install_file_if_changed(
                source / "sudo-shim.py", automation_dir / "sudo", "0755"
            )
            host_sudo = which("sudo")
            if host_sudo is None:
                raise DotfilesError(
                    "toshy-kanata-chain: sudo is not available on the host"
                )
            prefix = ":".join((
                os.fspath(automation_dir),
                "/run/wrappers/bin",
                "/usr/sbin",
                "/usr/bin",
                "/sbin",
                "/bin",
                "/usr/local/sbin",
                "/usr/local/bin",
                os.environ.get("PATH", ""),
            ))
            run(
                (sys.executable, automation, toshy_repo / "setup_toshy.py", *arguments),
                env={
                    "PATH": prefix,
                    "TOSHY_SUDO": os.fspath(host_sudo),
                    "TOSHY_SUDO_SHIM_DIR": os.fspath(automation_dir),
                },
            )
    _merge_toshy(config, merger, slices, source)
    restart = which("toshy-services-restart")
    if restart is not None:
        run((restart,), check=False)
    _hide_toshy_surfaces()
    console.print(
        "toshy-kanata-chain: merged dotfiles Toshy slices for Kanata virtual output"
    )
    return OperationResult(
        changed=True, msg="Reconciled the Toshy and Kanata device chain"
    )


class CheckReporter:
    def __init__(self) -> None:
        self.failures = 0

    def ok(self, message: str) -> None:
        console.print(f"OK: {message}")

    def fail(self, message: str) -> None:
        self.failures += 1
        error_console.print(f"FAIL: {message}")

    def warn(self, message: str) -> None:
        error_console.print(f"WARN: {message}")


def _unit_content(*arguments: str) -> str:
    return run(("systemctl", *arguments), check=False, capture=True).stdout


@app.command("toshy-kanata-check")
def toshy_kanata_check() -> OperationResult:
    """Verify the complete Kanata-to-Toshy device chain."""
    report = CheckReporter()
    repository = find_repo_root(Path(__file__))
    for scope, unit in (
        ((), "kanata-main.service"),
        (("--user",), "toshy-config.service"),
    ):
        active = (
            run(
                ("systemctl", *scope, "is-active", "--quiet", unit),
                check=False,
                capture=True,
            ).returncode
            == 0
        )
        (report.ok if active else report.fail)(
            f"{unit} is {'active' if active else 'not active'}"
        )
    kanata_unit = _unit_content("cat", "kanata-main.service")
    if "PrivateUsers=true" in kanata_unit:
        report.fail(
            "kanata-main.service uses PrivateUsers=true, which can hide input/uinput groups"
        )
    else:
        report.ok(
            "kanata-main.service does not isolate host input/uinput groups with PrivateUsers"
        )

    repo_config = repository / "dotfiles/dot_config/kanata/kanata.kbd"
    host_config = Path("/etc/kanata/kanata.kbd")
    if (
        repo_config.is_file()
        and host_config.is_file()
        and repo_config.read_bytes() == host_config.read_bytes()
    ):
        report.ok(f"{host_config} matches the dotfiles Kanata config")
    else:
        report.fail(f"{host_config} does not match {repo_config}")
    device = Path("/run/kanata-main/main")
    if device.is_symlink() and os.fspath(device.resolve()).startswith(
        "/dev/input/event"
    ):
        report.ok(f"{device} points to {device.resolve()}")
    else:
        report.fail(f"{device} is missing or points to an unexpected target")

    toshy_config = user_config_home() / "toshy/toshy_config.py"
    required_fragments = (
        "SLICE_MARK_START: keymapper_api",
        "SLICE_MARK_START: kbtype_override",
        "DOTFILES_TOSHY_ONLY_DEVICES",
        "/run/kanata-main/main",
        "dotfiles-kanata-main",
    )
    if toshy_config.is_file() and all(
        fragment in toshy_config.read_text() for fragment in required_fragments
    ):
        report.ok("Toshy config includes dotfiles Kanata device slice")
    else:
        report.fail("Toshy config does not include the dotfiles Kanata device slice")
    service = _unit_content("--user", "cat", "toshy-config.service")
    checks = (
        (
            "Environment=DOTFILES_TOSHY_ONLY_DEVICES=/run/kanata-main/main",
            "toshy-config.service restricts devices to /run/kanata-main/main",
        ),
        (
            "waiting for /run/kanata-main/main timed out",
            "toshy-config.service waits for the Kanata virtual device",
        ),
    )
    for fragment, message in checks:
        (report.ok if fragment in service else report.fail)(message)
    enabled = (
        run(
            (
                "systemctl",
                "--user",
                "is-enabled",
                "--quiet",
                "toshy-kanata-device.path",
            ),
            check=False,
            capture=True,
        ).returncode
        == 0
    )
    (report.ok if enabled else report.fail)("toshy-kanata-device.path is enabled")
    path_unit = _unit_content("--user", "cat", "toshy-kanata-device.path")
    (report.ok if "PathChanged=/run/kanata-main/main" in path_unit else report.fail)(
        "toshy-kanata-device.path watches the Kanata virtual device"
    )
    refresh = _unit_content("--user", "cat", "toshy-kanata-device.service")
    recover_fragments = (
        "reset-failed toshy-config.service",
        "[ -e /run/kanata-main/main ]",
        "restart toshy-config.service",
    )
    (
        report.ok
        if all(value in refresh for value in recover_fragments)
        else report.fail
    )("toshy-kanata-device.service can recover a failed Toshy service")
    input_remapper = (
        run(
            ("systemctl", "cat", "input-remapper.service"), check=False, capture=True
        ).returncode
        == 0
    )
    if input_remapper and os.environ.get("DOTFILES_KEEP_INPUT_REMAPPER") == "1":
        report.warn(
            "input-remapper.service exists and keep flag is set; skipping conflict check"
        )
    elif input_remapper:
        active = (
            run(
                ("systemctl", "is-active", "--quiet", "input-remapper.service"),
                check=False,
                capture=True,
            ).returncode
            == 0
        )
        (report.fail if active else report.ok)(
            "input-remapper.service is active and can compete with Kanata/Toshy"
            if active
            else "input-remapper.service is not active"
        )
    else:
        report.ok("input-remapper.service is not installed")
    if report.failures:
        raise DotfilesError(f"toshy-kanata-check: {report.failures} check(s) failed")
    console.print("toshy-kanata-check: setup looks consistent")
    return OperationResult(msg="Toshy and Kanata device chain is consistent")
