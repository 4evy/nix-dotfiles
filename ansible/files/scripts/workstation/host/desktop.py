import os
import platform
from pathlib import Path

from cyclopts import App

from workstation.automation import automation_check_mode
from workstation.automation_models import OperationResult
from workstation.console import console, error_console
from workstation.host import sushi_preview
from workstation.lib.commands import CommandResult, run, which
from workstation.lib.files import (
    ensure_directory,
    fresh_directory,
    install_file_if_changed,
    replace_directory,
    require_directory,
    require_file,
)
from workstation.lib.host import (
    HostRunner,
    enable_gnome_extensions,
    user_data_home,
    user_state_home,
)
from workstation.lib.paths import find_repo_root
from workstation.local.hyper_window_tiling_build import build_package

app = App(
    help="Configure Linux desktop integrations.",
    version_flags=[],
    result_action="return_none",
)


def _scope_run(
    host: HostRunner,
    installation: str,
    argv: tuple[str | os.PathLike[str], ...],
    *,
    check: bool = True,
    capture: bool = False,
) -> CommandResult:
    executor = host.root if installation == "system" else host.user
    return executor(argv, check=check, capture=capture)


def _flatpak_available(host: HostRunner) -> bool:
    return host.has_command("flatpak")


@app.command(name="flatpak-maintenance")
def flatpak_maintenance() -> OperationResult:
    """Repair user/system Flatpak installations and prune unused refs."""
    if automation_check_mode():
        return OperationResult(
            skipped=True, msg="Flatpak maintenance skipped in check mode"
        )
    host = HostRunner()
    if not _flatpak_available(host):
        console.print("flatpak-maintenance: flatpak is not available; skipping")
        return OperationResult(msg="Flatpak is not available")
    for installation in ("user", "system"):
        if installation == "system" and not Path("/var/lib/flatpak").is_dir():
            console.print(
                "flatpak-maintenance: system installation is not initialized; skipping"
            )
            continue
        console.print(f"flatpak-maintenance: repairing {installation} installation")
        repaired = _scope_run(
            host,
            installation,
            ("flatpak", "repair", f"--{installation}"),
            check=False,
        )
        if repaired.returncode != 0:
            error_console.print(
                f"flatpak-maintenance: repair failed for {installation} installation"
            )
        console.print(
            f"flatpak-maintenance: pruning unused refs from {installation} installation"
        )
        pruned = _scope_run(
            host,
            installation,
            ("flatpak", "uninstall", f"--{installation}", "--unused", "-y"),
            check=False,
        )
        if pruned.returncode != 0:
            error_console.print(
                f"flatpak-maintenance: unused cleanup failed for {installation} installation"
            )
    return OperationResult(msg="Completed Flatpak maintenance")


def _installation_has_flathub(host: HostRunner, installation: str) -> bool:
    result = _scope_run(
        host,
        installation,
        ("flatpak", "remotes", f"--{installation}", "--columns=name"),
        check=False,
        capture=True,
    )
    return "flathub" in result.stdout.splitlines()


def _runtime_installed(host: HostRunner, installation: str, runtime: str) -> bool:
    return (
        _scope_run(
            host,
            installation,
            ("flatpak", "info", f"--{installation}", runtime),
            check=False,
            capture=True,
        ).returncode
        == 0
    )


def _install_nvidia_runtimes(host: HostRunner, installation: str) -> bool:
    drivers_result = _scope_run(
        host,
        installation,
        ("flatpak", "--gl-drivers"),
        check=False,
        capture=True,
    )
    drivers = [
        line
        for line in drivers_result.stdout.splitlines()
        if line.startswith("nvidia-")
    ]
    if not drivers:
        console.print("flatpak-nvidia: no active NVIDIA Flatpak GL driver; skipping")
        return False
    if not _installation_has_flathub(host, installation):
        console.print(
            f"flatpak-nvidia: {installation} installation has no flathub remote; skipping"
        )
        return False

    changed = False
    for driver in drivers:
        runtime = f"org.freedesktop.Platform.GL.{driver}//1.4"
        if _runtime_installed(host, installation, runtime):
            console.print(
                f"flatpak-nvidia: {runtime} already installed in {installation} installation"
            )
            continue
        console.print(
            f"flatpak-nvidia: ensuring {runtime} in {installation} installation"
        )
        _scope_run(
            host,
            installation,
            (
                "flatpak",
                "install",
                f"--{installation}",
                "--noninteractive",
                "flathub",
                runtime,
            ),
        )
        changed = True

    remote_runtimes = _scope_run(
        host,
        installation,
        (
            "flatpak",
            "remote-ls",
            f"--{installation}",
            "flathub",
            "--runtime",
            "--columns=application,branch",
        ),
        check=False,
        capture=True,
    )
    vaapi = "org.freedesktop.Platform.VAAPI.nvidia"
    for line in remote_runtimes.stdout.splitlines():
        fields = line.split()
        if len(fields) < 2 or fields[0] != vaapi:
            continue
        runtime = f"{vaapi}//{fields[1]}"
        if _runtime_installed(host, installation, runtime):
            console.print(
                f"flatpak-nvidia: {runtime} already installed in {installation} installation"
            )
            continue
        console.print(
            f"flatpak-nvidia: ensuring {runtime} in {installation} installation"
        )
        _scope_run(
            host,
            installation,
            (
                "flatpak",
                "install",
                f"--{installation}",
                "--noninteractive",
                "flathub",
                runtime,
            ),
        )
        changed = True
    return changed


@app.command(name="flatpak-nvidia")
def flatpak_nvidia() -> OperationResult:
    """Install NVIDIA GL and VA-API runtimes for both Flatpak installations."""
    host = HostRunner()
    if not _flatpak_available(host):
        console.print("flatpak-nvidia: flatpak is not available; skipping")
        return OperationResult(msg="Flatpak is not available")
    if automation_check_mode():
        return OperationResult(
            changed=True, msg="Would reconcile NVIDIA Flatpak runtimes"
        )
    changed = _install_nvidia_runtimes(host, "user")
    changed = _install_nvidia_runtimes(host, "system") or changed
    return OperationResult(
        changed=changed,
        msg=(
            "Installed missing NVIDIA Flatpak runtimes"
            if changed
            else "NVIDIA Flatpak runtimes are current"
        ),
    )


def _gnome_available() -> bool:
    return which("gnome-shell") is not None and which("gnome-extensions") is not None


def _install_gnome(build_root: Path) -> bool:
    label = "hyper-window-tiling"
    uuid = "hyper-window-tiling@4evy.local"
    if not _gnome_available():
        console.print(f"{label}: GNOME Shell is not available; skipping")
        return False
    source = build_root / "gnome" / uuid
    require_file(source / "metadata.json")
    require_file(source / "extension.js")
    schemas = require_directory(source / "schemas")
    if which("glib-compile-schemas") is not None:
        run(("glib-compile-schemas", schemas))
    destination = user_data_home() / "gnome-shell/extensions" / uuid
    ensure_directory(destination.parent)
    replace_directory(source, destination)
    run(("gnome-extensions", "enable", uuid), check=False, capture=True)
    enable_gnome_extensions(uuid)
    schema_env = {"GSETTINGS_SCHEMA_DIR": os.fspath(destination / "schemas")}
    key_schema = "org.gnome.shell.extensions.hyper-window-tiling"
    keys = {
        "move-up": "['<Super><Control><Alt><Shift>w']",
        "move-left": "['<Super><Control><Alt><Shift>a']",
        "move-down": "['<Super><Control><Alt><Shift>s']",
        "move-right": "['<Super><Control><Alt><Shift>d']",
        "move-max-almost": "['<Super><Control><Alt><Shift>Return']",
        "move-max": "['<Super><Control><Alt><Shift>backslash']",
    }
    for key, value in keys.items():
        run(("gsettings", "set", key_schema, key, value), env=schema_env)
    wm_schema = "org.gnome.desktop.wm.keybindings"
    for key in (
        "switch-to-workspace-left",
        "switch-to-workspace-right",
        "move-to-workspace-left",
        "move-to-workspace-right",
    ):
        run(("gsettings", "set", wm_schema, key, "[]"))
    console.print(f"{label}: installed GNOME extension {uuid}")
    return True


def _kde_available() -> bool:
    return (
        which("kwin_wayland") is not None
        or which("plasmashell") is not None
        or Path("/usr/share/plasma").is_dir()
        or Path("/usr/share/wayland-sessions/plasma.desktop").is_file()
    )


def _first_command(*names: str) -> Path | None:
    for name in names:
        candidate = which(name)
        if candidate is not None:
            return candidate
    return None


def _install_kde(build_root: Path) -> bool:
    if not _kde_available():
        console.print("hyper-window-tiling: KDE Plasma is not available; skipping")
        return False
    source = build_root / "kde/hyper-window-tiling"
    require_file(source / "metadata.json")
    require_file(source / "contents/code/main.js")
    state_dir = user_state_home() / "dotfiles/hyper-window-tiling"
    install_source = fresh_directory(state_dir / "kwin-script/hyper-window-tiling")
    install_file_if_changed(source / "metadata.json", install_source / "metadata.json")
    install_file_if_changed(
        source / "contents/code/main.js", install_source / "contents/code/main.js"
    )

    installed = False
    kpackage = _first_command("kpackagetool6", "kpackagetool5")
    if kpackage is not None:
        upgraded = run(
            (kpackage, "--type", "KWin/Script", "--upgrade", install_source),
            check=False,
            capture=True,
        )
        if upgraded.returncode == 0:
            installed = True
        else:
            installed = (
                run(
                    (kpackage, "--type", "KWin/Script", "--install", install_source),
                    check=False,
                    capture=True,
                ).returncode
                == 0
            )
    if not installed:
        for destination in (
            user_data_home() / "kwin/scripts/hyper-window-tiling",
            user_data_home() / "kwin-wayland/scripts/hyper-window-tiling",
        ):
            replace_directory(install_source, destination)
    kwriteconfig = _first_command("kwriteconfig6", "kwriteconfig5")
    if kwriteconfig is not None:
        run(
            (
                kwriteconfig,
                "--file",
                "kwinrc",
                "--group",
                "Plugins",
                "--key",
                "hyper-window-tilingEnabled",
                "true",
            ),
            check=False,
            capture=True,
        )
    qdbus = _first_command("qdbus6", "qdbus")
    if qdbus is not None:
        run((qdbus, "org.kde.KWin", "/KWin", "reconfigure"), check=False, capture=True)
    console.print("hyper-window-tiling: installed KDE script")
    return True


@app.command(name="hyper-window-tiling")
def hyper_window_tiling() -> OperationResult:
    """Build and install the GNOME and KDE window-tiling integrations."""
    if automation_check_mode():
        changed = _gnome_available() or _kde_available()
        return OperationResult(
            changed=changed,
            msg=(
                "Would install window-tiling integrations"
                if changed
                else "No supported desktop environment is available"
            ),
        )
    repository = find_repo_root(Path(__file__))
    build_root = build_package(
        source_directory=repository / "dotfiles",
        home_directory=Path.home(),
        os_name=platform.system(),
    )
    if build_root is None:
        return OperationResult(msg="No window-tiling package was built")
    changed = _install_gnome(build_root)
    changed = _install_kde(build_root) or changed
    return OperationResult(
        changed=changed,
        msg=(
            "Installed window-tiling integrations"
            if changed
            else "No supported desktop environment is available"
        ),
    )


@app.command(name="sushi-preview")
def install_sushi_preview() -> OperationResult:
    """Build and install the pinned GNOME Sushi Flatpak."""
    return sushi_preview.install()
