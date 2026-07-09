from __future__ import annotations

import hashlib
import json
import os
import tempfile
from pathlib import Path

import tomlkit
import typer
from tomlkit.exceptions import ParseError

from workstation.automation import automation_check_mode
from workstation.automation_models import OperationResult
from workstation.console import error_console
from workstation.errors import DotfilesError
from workstation.lib.commands import output, run, which
from workstation.lib.files import (
    ensure_directory,
    install_file_if_changed,
    require_file,
    write_if_changed,
)
from workstation.lib.host import HostRunner, user_config_home, user_data_home
from workstation.lib.paths import find_repo_root
from workstation.lib.retry import wait_until

app = typer.Typer(no_args_is_help=True, pretty_exceptions_enable=False)


def _source_root() -> Path:
    return find_repo_root(Path(__file__)) / "ansible/files/scripts/host/apps"


def _service_context(service: str) -> str:
    pid = output(("systemctl", "show", "-P", "MainPID", service), check=False)
    if not pid or pid == "0":
        return ""
    lines = output(("ps", "-p", pid, "-o", "label="), check=False).splitlines()
    return lines[0].strip() if lines else ""


def _policy_hash(directory: Path, names: tuple[str, ...]) -> str:
    digest = hashlib.sha256()
    for name in names:
        path = require_file(directory / name)
        digest.update(name.encode())
        digest.update(b"\0")
        digest.update(path.read_bytes())
    return digest.hexdigest()


def _selinux_enabled() -> bool:
    return (
        which("getenforce") is not None
        and output(("getenforce",), check=False) != "Disabled"
    )


def _module_installed(name: str) -> bool:
    modules = output(("semodule", "-l"), check=False)
    return any(line.split()[0:1] == [name] for line in modules.splitlines())


def _build_install_policy(
    *,
    policy_dir: Path,
    module: str,
    source_names: tuple[str, ...],
    hash_file: Path,
    state_mode: str,
) -> bool:
    policy_makefile = Path("/usr/share/selinux/devel/Makefile")
    if not policy_makefile.is_file():
        error_console.print(
            f"{module}: selinux-policy-devel is not installed; add it to the "
            "Spectrum image to build the SELinux policy"
        )
        return False
    digest = _policy_hash(policy_dir, source_names)
    needs_install = not _module_installed(module) or not hash_file.is_file()
    if hash_file.is_file() and hash_file.read_text(encoding="utf-8").strip() != digest:
        needs_install = True
    if not needs_install:
        return False
    with tempfile.TemporaryDirectory(prefix=f"{module}-selinux-") as temporary:
        build = Path(temporary)
        for name in source_names:
            install_file_if_changed(policy_dir / name, build / name)
        run(("make", "-C", build, "-f", policy_makefile, f"{module}.pp"))
        run(("semodule", "-i", build / f"{module}.pp"))
    ensure_directory(hash_file.parent, state_mode)
    write_if_changed(hash_file, digest + "\n")
    return True


def _restore(paths: tuple[tuple[Path, bool], ...]) -> None:
    if which("restorecon") is None:
        return
    for path, recursive in paths:
        if path.exists():
            arguments: tuple[str | os.PathLike[str], ...] = (
                ("restorecon", "-R", path) if recursive else ("restorecon", path)
            )
            run(arguments, check=False)


def _tailscale_ssh_sessions_active() -> bool:
    pid = output(("systemctl", "show", "-P", "MainPID", "tailscaled"), check=False)
    if not pid or pid == "0":
        return False
    return (
        run(
            ("pgrep", "-P", pid, "-f", "tailscaled be-child ssh"),
            check=False,
            capture=True,
        ).returncode
        == 0
    )


def _configure_tailscale_selinux() -> None:
    if not _selinux_enabled():
        return
    policy_dir = _source_root() / "tailscale-selinux"
    sources = ("tailscaled.te", "tailscaled.fc", "tailscaled.if")
    for name in sources:
        require_file(policy_dir / name)
    hash_file = Path("/var/lib/tailscale/dotfiles-selinux-policy.sha256")
    digest = _policy_hash(policy_dir, sources)
    policy_change = not _module_installed("tailscaled") or not hash_file.is_file()
    if hash_file.is_file() and hash_file.read_text(encoding="utf-8").strip() != digest:
        policy_change = True
    dropin = Path("/etc/systemd/system/tailscaled.service.d/10-selinux-context.conf")
    desired = "[Service]\nSELinuxContext=system_u:system_r:tailscaled_t:s0\n"
    dropin_change = (
        not dropin.is_file() or dropin.read_text(encoding="utf-8") != desired
    )
    allow_reload = os.environ.get("DOTFILES_TAILSCALE_ALLOW_LIVE_RELOAD") == "1"
    if (
        (policy_change or dropin_change)
        and not allow_reload
        and _tailscale_ssh_sessions_active()
    ):
        error_console.print(
            "tailscale-bluefin: active Tailscale SSH session detected; deferring "
            "SELinux policy/drop-in changes to avoid interrupting it"
        )
        error_console.print(
            "tailscale-bluefin: rerun locally, after disconnecting SSH, or set "
            "DOTFILES_TAILSCALE_ALLOW_LIVE_RELOAD=1 to force it"
        )
        return
    installed = _build_install_policy(
        policy_dir=policy_dir,
        module="tailscaled",
        source_names=sources,
        hash_file=hash_file,
        state_mode="0700",
    )
    if installed:
        _restore(
            tuple(
                (Path(path), recursive)
                for path, recursive in (
                    ("/usr/bin/tailscaled", False),
                    ("/usr/sbin/tailscaled", False),
                    ("/usr/lib/systemd/system/tailscaled.service", False),
                    ("/etc/systemd/system/tailscaled.service", False),
                    ("/var/lib/tailscale", True),
                    ("/var/cache/tailscale", True),
                    ("/run/tailscale", True),
                    ("/var/run/tailscale", True),
                )
            )
        )
    if dropin_change:
        write_if_changed(dropin, desired)
        run(("systemctl", "daemon-reload"))
    active = (
        run(
            ("systemctl", "is-active", "--quiet", "tailscaled"),
            check=False,
            capture=True,
        ).returncode
        == 0
    )
    expected = "system_u:system_r:tailscaled_t:s0"
    restart = not active or dropin_change or _service_context("tailscaled") != expected
    if restart and not allow_reload and _tailscale_ssh_sessions_active():
        error_console.print(
            "tailscale-bluefin: active Tailscale SSH session detected; "
            "deferring tailscaled restart to avoid interrupting it"
        )
        return
    if (
        restart
        and run(("systemctl", "restart", "tailscaled"), check=False).returncode != 0
    ):
        dropin.unlink(missing_ok=True)
        run(("systemctl", "daemon-reload"))
        run(("systemctl", "reset-failed", "tailscaled"), check=False)
        run(("systemctl", "start", "tailscaled"), check=False)
        raise DotfilesError(
            "tailscale-bluefin: confined tailscaled restart failed; removed "
            "SELinuxContext drop-in and restarted the unconfined service"
        )
    context = _service_context("tailscaled")
    if context != expected:
        raise DotfilesError(
            "tailscale-bluefin: tailscaled is not running in the expected "
            f"SELinux context; got: {context or 'not running'}"
        )


@app.command("tailscale-system", hidden=True)
def tailscale_system() -> None:
    if os.geteuid() != 0:
        raise DotfilesError("tailscale-system must run as root")
    if which("tailscale") is None or which("tailscaled") is None:
        error_console.print(
            "tailscale-bluefin: Tailscale is not installed; add it to the "
            "Spectrum image and switch to the rebuilt image"
        )
        return
    run(("systemctl", "enable", "tailscaled"))
    run(("systemctl", "start", "tailscaled"), check=False)
    _configure_tailscale_selinux()


def _validate_tailscale_selinux(host: HostRunner) -> None:
    if (
        output(("getenforce",), check=False) != "Enforcing"
        or which("tailscale") is None
    ):
        return
    preferences = host.root(("tailscale", "debug", "prefs"), check=False, capture=True)
    try:
        ssh_enabled = bool(json.loads(preferences.stdout).get("RunSSH"))
    except json.JSONDecodeError, AttributeError:
        ssh_enabled = False
    if not ssh_enabled:
        return
    context = host.root_output(
        ("systemctl", "show", "-P", "MainPID", "tailscaled"), check=False
    )
    if context and context != "0":
        context = host.root_output(("ps", "-p", context, "-o", "label="), check=False)
    if context != "system_u:system_r:tailscaled_t:s0":
        raise DotfilesError(
            "tailscale-bluefin: Tailscale SSH is enabled under enforcing SELinux, "
            f"but tailscaled is running as {context or 'not running'}"
        )
    error_console.print(
        "tailscale-bluefin: Tailscale SSH SELinux policy is installed; tailscale "
        "status may still show the upstream generic SELinux warning"
    )


@app.command("tailscale-bluefin")
def tailscale_bluefin() -> OperationResult:
    """Configure Tailscale and its SELinux domain on immutable Fedora hosts."""
    if automation_check_mode():
        return OperationResult(
            changed=True, msg="Would reconcile Tailscale host integration"
        )
    host = HostRunner()
    host.root_python("host", "apps", "tailscale-system")
    if which("tailscale") is None:
        error_console.print(
            "tailscale-bluefin: tailscale is not available; add it to the Spectrum image"
        )
        return OperationResult(
            changed=True,
            msg="Reconciled host policy; Tailscale executable is unavailable",
        )
    ready = wait_until(
        lambda: (
            run(("tailscale", "status"), check=False, capture=True).returncode == 0
            or host.root(("tailscale", "status"), check=False, capture=True).returncode
            == 0
        ),
        attempts=10,
        interval=1,
    )
    if ready:
        user_result = run(
            ("tailscale", "set", "--auto-update=false"), check=False, capture=True
        )
        if (
            user_result.returncode != 0
            and host.root(
                ("tailscale", "set", "--auto-update=false"), check=False, capture=True
            ).returncode
            != 0
        ):
            error_console.print(
                "tailscale-bluefin: could not disable Tailscale auto-update; "
                "keep updates managed by the Spectrum image"
            )
    else:
        error_console.print(
            "tailscale-bluefin: tailscale is installed but not authenticated; "
            "run tailscale up on this host"
        )
    _validate_tailscale_selinux(host)
    return OperationResult(changed=True, msg="Reconciled Tailscale host integration")


APPARMOR_PROFILE = """abi <abi/4.0>,
include <tunables/global>

profile dotfiles-rustdesk /usr/bin/rustdesk flags=(default_allow) {
  capability,
  network,
  dbus,
  unix,
  signal,
  ptrace,
  /dev/uinput rw,
  /dev/input/** rw,
  /run/user/*/bus rw,
  /run/user/*/pipewire-0 rw,
  /run/user/*/wayland-* rw,
  /tmp/.X11-unix/* rw,
  /usr/share/rustdesk/** rix,
  owner @{HOME}/.config/rustdesk/** rwk,
  owner @{HOME}/.local/share/rustdesk/** rwk,
  include if exists <local/dotfiles-rustdesk>
}

profile dotfiles-rustdesk-share /usr/share/rustdesk/rustdesk flags=(default_allow) {
  capability,
  network,
  dbus,
  unix,
  signal,
  ptrace,
  /dev/uinput rw,
  /dev/input/** rw,
  /run/user/*/bus rw,
  /run/user/*/pipewire-0 rw,
  /run/user/*/wayland-* rw,
  /tmp/.X11-unix/* rw,
  /usr/bin/rustdesk rix,
  /usr/share/rustdesk/** rix,
  owner @{HOME}/.config/rustdesk/** rwk,
  owner @{HOME}/.local/share/rustdesk/** rwk,
  include if exists <local/dotfiles-rustdesk-share>
}
"""


def _configure_rustdesk_apparmor() -> None:
    if os.environ.get("DOTFILES_RUSTDESK_APPARMOR", "1") == "0":
        return
    enabled = Path("/sys/module/apparmor/parameters/enabled")
    if not enabled.is_file() or not enabled.read_text(
        encoding="utf-8"
    ).upper().startswith("Y"):
        return
    if which("apparmor_parser") is None:
        return
    profile = Path("/etc/apparmor.d/dotfiles-rustdesk")
    write_if_changed(profile, APPARMOR_PROFILE)
    run(("apparmor_parser", "-r", profile))


def merge_rustdesk_options(config: Path) -> None:
    ensure_directory(config.parent)
    try:
        document = (
            tomlkit.parse(config.read_text(encoding="utf-8"))
            if config.is_file()
            else tomlkit.document()
        )
    except ParseError as error:
        raise DotfilesError(f"invalid RustDesk TOML configuration: {config}") from error
    options = document.get("options")
    if options is None:
        options = tomlkit.table()
        document["options"] = options
    if not isinstance(options, dict):
        raise DotfilesError(f"RustDesk options is not a TOML table: {config}")
    options["direct-server"] = "Y"
    options["direct-access-port"] = "21118"
    write_if_changed(config, tomlkit.dumps(document))


def _configure_rustdesk_selinux() -> None:
    if (
        os.environ.get("DOTFILES_RUSTDESK_SELINUX", "1") == "0"
        or not _selinux_enabled()
    ):
        return
    policy_dir = _source_root() / "rustdesk-selinux"
    sources = ("rustdesk.te", "rustdesk.fc", "rustdesk.if")
    installed = _build_install_policy(
        policy_dir=policy_dir,
        module="rustdesk",
        source_names=sources,
        hash_file=Path("/var/lib/rustdesk/dotfiles-selinux-policy.sha256"),
        state_mode="0700",
    )
    if installed:
        _restore(
            tuple(
                (Path(path), True)
                for path in (
                    "/usr/bin/rustdesk",
                    "/usr/share/rustdesk/rustdesk",
                    "/etc/systemd/system/rustdesk.service",
                    "/usr/lib/systemd/system/rustdesk.service",
                    "/var/lib/rustdesk",
                    "/run/rustdesk.pid",
                    "/var/run/rustdesk.pid",
                )
            )
        )
    dropin = Path("/etc/systemd/system/rustdesk.service.d/10-selinux-context.conf")
    desired = "[Service]\nSELinuxContext=system_u:system_r:rustdesk_t:s0\n"
    changed = not dropin.is_file() or dropin.read_text(encoding="utf-8") != desired
    if changed:
        write_if_changed(dropin, desired)
        run(("systemctl", "daemon-reload"))
    active = (
        run(
            ("systemctl", "is-active", "--quiet", "rustdesk.service"),
            check=False,
            capture=True,
        ).returncode
        == 0
    )
    expected = "system_u:system_r:rustdesk_t:s0"
    if (
        active
        and (installed or changed or _service_context("rustdesk.service") != expected)
        and (
            run(("systemctl", "restart", "rustdesk.service"), check=False).returncode
            != 0
        )
    ):
        dropin.unlink(missing_ok=True)
        run(("systemctl", "daemon-reload"))
        run(("systemctl", "reset-failed", "rustdesk.service"), check=False)
        run(("systemctl", "start", "rustdesk.service"), check=False)
        raise DotfilesError(
            "rustdesk-tailscale: rustdesk restart failed under rustdesk_t; "
            "removed SELinuxContext drop-in and restarted unconfined"
        )
    if active and _service_context("rustdesk.service") != expected:
        raise DotfilesError(
            "rustdesk-tailscale: rustdesk is not running in the expected SELinux "
            f"context; got: {_service_context('rustdesk.service') or 'not running'}"
        )


@app.command("rustdesk-system", hidden=True)
def rustdesk_system() -> None:
    if os.geteuid() != 0:
        raise DotfilesError("rustdesk-system must run as root")
    _configure_rustdesk_apparmor()
    _configure_rustdesk_selinux()
    if run(("rpm", "-q", "rustdesk"), check=False, capture=True).returncode == 0:
        merge_rustdesk_options(Path.home() / ".config/rustdesk/RustDesk2.toml")
        run(("systemctl", "restart", "rustdesk.service"))


def _prepare_rustdesk_wayland() -> None:
    if os.environ.get("XDG_SESSION_TYPE") != "wayland":
        return
    if which("systemctl") is not None:
        run(
            (
                "systemctl",
                "--user",
                "reset-failed",
                "xdg-desktop-portal.service",
                "xdg-desktop-portal-gnome.service",
                "xdg-desktop-portal-gtk.service",
                "pipewire.service",
                "wireplumber.service",
            ),
            check=False,
            capture=True,
        )
        run(
            (
                "systemctl",
                "--user",
                "start",
                "xdg-desktop-portal.service",
                "pipewire.service",
                "wireplumber.service",
            ),
            check=False,
            capture=True,
        )
    runtime = Path(os.environ.get("XDG_RUNTIME_DIR", "/nonexistent"))
    if not (runtime / "bus").is_socket():
        error_console.print(
            "rustdesk-tailscale: Wayland session bus is not available; RustDesk "
            "portal capture will fail until the user session is healthy"
        )
    if not (runtime / "pipewire-0").is_socket():
        error_console.print(
            "rustdesk-tailscale: PipeWire socket is not available; RustDesk "
            "Wayland screen capture will fail"
        )
    if not Path("/dev/uinput").exists():
        error_console.print(
            "rustdesk-tailscale: /dev/uinput is missing; RustDesk Wayland "
            "keyboard/mouse fallback will not work"
        )


def _install_rustdesk_desktop() -> None:
    source = require_file(_source_root() / "rustdesk.desktop")
    applications = ensure_directory(user_data_home() / "applications")
    desktop = applications / "rustdesk.desktop"
    install_file_if_changed(source, desktop)
    autostart = user_config_home() / "autostart/rustdesk.desktop"
    if autostart.is_file():
        write_if_changed(
            autostart,
            desktop.read_text().rstrip("\n") + "\nX-GNOME-Autostart-enabled=true\n",
        )
    if which("update-desktop-database") is not None:
        run(("update-desktop-database", applications), check=False, capture=True)


@app.command("rustdesk-tailscale")
def rustdesk_tailscale() -> OperationResult:
    """Configure native RustDesk for direct Tailscale access and Wayland capture."""
    if which("rustdesk") is None:
        error_console.print(
            "rustdesk-tailscale: rustdesk is not installed; add it to the Spectrum image"
        )
        return OperationResult(msg="RustDesk is not installed")
    if automation_check_mode():
        return OperationResult(
            changed=True, msg="Would reconcile RustDesk Tailscale integration"
        )
    host = HostRunner()
    host.root_python("host", "apps", "rustdesk-system")
    _prepare_rustdesk_wayland()
    if which("flatpak") is not None:
        run(
            (
                "flatpak",
                "uninstall",
                "--user",
                "--noninteractive",
                "com.rustdesk.RustDesk",
            ),
            check=False,
            capture=True,
        )
        host.root(
            (
                "flatpak",
                "uninstall",
                "--system",
                "--noninteractive",
                "com.rustdesk.RustDesk",
            ),
            check=False,
            capture=True,
        )
    _install_rustdesk_desktop()
    merge_rustdesk_options(user_config_home() / "rustdesk/RustDesk2.toml")
    if which("tailscale") is not None:
        host.root(("systemctl", "enable", "--now", "tailscaled"), check=False)
        if run(("tailscale", "status"), check=False, capture=True).returncode != 0:
            error_console.print(
                "rustdesk-tailscale: tailscale is installed but not authenticated; "
                "run tailscale up on this host"
            )
    else:
        error_console.print(
            "rustdesk-tailscale: tailscale is not installed; install the tailscale "
            "host tool before relying on direct IP access"
        )
    return OperationResult(
        changed=True, msg="Reconciled RustDesk Tailscale integration"
    )
