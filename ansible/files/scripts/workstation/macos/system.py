from __future__ import annotations

import hashlib
import os
import tempfile
import time
from pathlib import Path
from typing import Annotated

import typer

from workstation.automation import automation_check_mode
from workstation.automation_models import OperationResult
from workstation.console import error_console
from workstation.errors import DotfilesError
from workstation.lib.commands import require_commands, run
from workstation.lib.files import (
    ensure_directory,
    install_file_if_changed,
    require_executable,
    require_file,
)
from workstation.lib.http import download
from workstation.lib.paths import find_repo_root
from workstation.lib.retry import wait_until
from workstation.lib.templates import render_template

KARABINER_VERSION = "6.2.0"
KARABINER_SHA256 = "9e8c46239f0748161241e42444857901224e5c82f5b58a1731df4c70bf0736a8"
KARABINER_LABEL = "org.pqrs.service.daemon.Karabiner-VirtualHIDDevice-Daemon"
KANATA_LABEL = "dev.4evy.kanata"


def _require_root(command: str) -> None:
    if os.geteuid() != 0:
        raise DotfilesError(f"{command}: this command must run as root")


def _source_root() -> Path:
    return find_repo_root(Path(__file__)) / "ansible/files/scripts/macos"


def _chown_root(*paths: Path) -> None:
    run(("chown", "-R", "root:wheel", *paths))


def _bootout(plist: Path) -> None:
    run(("launchctl", "bootout", "system", plist), check=False, capture=True)


def _karabiner_paths() -> tuple[Path, Path, Path]:
    manager = Path(
        "/Applications/.Karabiner-VirtualHIDDevice-Manager.app/Contents/MacOS/"
        "Karabiner-VirtualHIDDevice-Manager"
    )
    daemon = Path(
        "/Library/Application Support/org.pqrs/Karabiner-DriverKit-VirtualHIDDevice/"
        "Applications/Karabiner-VirtualHIDDevice-Daemon.app/Contents/MacOS/"
        "Karabiner-VirtualHIDDevice-Daemon"
    )
    plist = Path(f"/Library/LaunchDaemons/{KARABINER_LABEL}.plist")
    return manager, daemon, plist


def configure_karabiner_vhid() -> OperationResult:
    """Install and activate the pinned Karabiner VirtualHID DriverKit daemon."""
    _require_root("karabiner-vhid")
    require_commands("installer", "launchctl", "chown")
    manager, daemon, plist = _karabiner_paths()
    if automation_check_mode():
        running = (
            run(
                ("launchctl", "print", f"system/{KARABINER_LABEL}"),
                check=False,
                capture=True,
            ).returncode
            == 0
        )
        current = all(path.is_file() for path in (manager, daemon, plist)) and running
        return OperationResult(
            changed=not current,
            msg=(
                "Karabiner VirtualHID is current"
                if current
                else "Would install or activate Karabiner VirtualHID"
            ),
        )
    if not (manager.is_file() and os.access(manager, os.X_OK)) or not (
        daemon.is_file() and os.access(daemon, os.X_OK)
    ):
        package = f"Karabiner-DriverKit-VirtualHIDDevice-{KARABINER_VERSION}.pkg"
        url = (
            "https://github.com/pqrs-org/Karabiner-DriverKit-VirtualHIDDevice/"
            f"releases/download/v{KARABINER_VERSION}/{package}"
        )
        with tempfile.TemporaryDirectory(prefix="karabiner-vhid-") as temporary:
            package_path = Path(temporary) / package
            download(url, package_path)
            digest = hashlib.sha256(package_path.read_bytes()).hexdigest()
            if digest != KARABINER_SHA256:
                raise DotfilesError(f"karabiner-vhid: checksum mismatch for {package}")
            run(("installer", "-pkg", package_path, "-target", "/"))
    require_executable(manager)
    require_executable(daemon)
    run((manager, "forceActivate"))

    ensure_directory("/var/log/karabiner", "0755")
    _bootout(plist)
    render_template(
        _source_root() / "templates/karabiner-vhid.plist.in",
        plist,
        {"LABEL": KARABINER_LABEL, "DAEMON": daemon},
    )
    run(("chown", "root:wheel", plist))
    run(("launchctl", "bootstrap", "system", plist))
    run(("launchctl", "enable", f"system/{KARABINER_LABEL}"))
    run(("launchctl", "kickstart", "-k", f"system/{KARABINER_LABEL}"))
    return OperationResult(changed=True, msg="Activated Karabiner VirtualHID")


def _ensure_virtual_hid() -> None:
    state = run(
        ("launchctl", "print", f"system/{KARABINER_LABEL}"),
        check=False,
        capture=True,
    )
    if state.returncode != 0:
        configure_karabiner_vhid()
    else:
        run(
            ("launchctl", "kickstart", "-k", f"system/{KARABINER_LABEL}"),
            check=False,
            capture=True,
        )

    def daemon_running() -> bool:
        state = run(
            ("launchctl", "print", f"system/{KARABINER_LABEL}"),
            check=False,
            capture=True,
        )
        return "state = running" in state.stdout

    if not wait_until(daemon_running, attempts=20, interval=0.5):
        raise DotfilesError(f"kanata: {KARABINER_LABEL} did not reach running state")


def _ensure_signing_identity(identity: str, keychain: Path) -> bool:
    identities = run(
        ("security", "find-identity", "-v", "-p", "codesigning", keychain),
        check=False,
        capture=True,
    ).stdout
    if identity in identities:
        return True
    config = f"""[ req ]
distinguished_name = dn
x509_extensions = v3_req
prompt = no
[ dn ]
CN = {identity}
[ v3_req ]
keyUsage = critical, digitalSignature
extendedKeyUsage = codeSigning
basicConstraints = critical, CA:false
"""
    with tempfile.TemporaryDirectory(prefix="kanata-codesign-") as temporary:
        root = Path(temporary)
        openssl_config = root / "kanata-codesign-openssl.cnf"
        openssl_config.write_text(config)
        key = root / "kanata.key"
        certificate = root / "kanata.crt"
        archive = root / "kanata.p12"
        run((
            "openssl",
            "req",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            key,
            "-x509",
            "-days",
            "3650",
            "-out",
            certificate,
            "-config",
            openssl_config,
        ))
        run((
            "openssl",
            "pkcs12",
            "-export",
            "-inkey",
            key,
            "-in",
            certificate,
            "-out",
            archive,
            "-passout",
            "pass:kanata-local",
        ))
        run((
            "security",
            "import",
            archive,
            "-k",
            keychain,
            "-P",
            "kanata-local",
            "-T",
            "/usr/bin/codesign",
        ))
        run((
            "security",
            "add-trusted-cert",
            "-d",
            "-r",
            "trustRoot",
            "-p",
            "codeSign",
            "-k",
            keychain,
            certificate,
        ))
    return (
        identity
        in run(
            ("security", "find-identity", "-v", "-p", "codesigning", keychain),
            check=False,
            capture=True,
        ).stdout
    )


def _stop_daemon(label: str) -> None:
    _bootout(Path(f"/Library/LaunchDaemons/{label}.plist"))


def configure_kanata(
    config: Annotated[Path, typer.Argument(help="Kanata configuration file")],
) -> OperationResult:
    """Install, sign, and launch Kanata with VirtualHID support."""
    _require_root("kanata")
    config = require_file(config)
    require_commands("chown", "codesign", "launchctl", "openssl", "security")
    if automation_check_mode():
        return OperationResult(
            changed=True, msg="Would reconcile the macOS Kanata launch daemon"
        )
    source = _source_root()
    kanata_bin = require_executable("/opt/homebrew/bin/kanata")
    logitech = require_file(source / "logitech-platform.py")
    label = KANATA_LABEL
    app = Path("/Applications/Kanata.app")
    app_bin = app / "Contents/MacOS/kanata"
    info_plist = app / "Contents/Info.plist"
    identity = "Kanata Local Code Signing"
    keychain = Path("/Library/Keychains/System.keychain")

    _ensure_virtual_hid()
    ensure_directory(app_bin.parent, "0755")
    install_file_if_changed(kanata_bin, app_bin, "0755")
    render_template(
        source / "templates/kanata-app-info.plist.in",
        info_plist,
        {"BUNDLE_IDENTIFIER": label},
    )
    _chown_root(app)
    if _ensure_signing_identity(identity, keychain):
        run((
            "codesign",
            "--force",
            "--keychain",
            keychain,
            "--sign",
            identity,
            app,
        ))
    else:
        run(("codesign", "--force", "--sign", "-", app))
    run((
        "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister",
        "-f",
        app,
    ))
    _stop_daemon(label)
    time.sleep(0.5)
    result = run(
        (
            "/usr/bin/python3",
            logitech,
            "--product-id",
            "0xB377",
            "--product-name",
            "Pebble K380s",
            "--platform",
            "0",
        ),
        check=False,
    )
    if result.returncode != 0:
        error_console.print(
            "kanata: warning: failed to configure Pebble K380s non-macOS mode; "
            "continuing with Kanata setup"
        )

    plist = Path(f"/Library/LaunchDaemons/{label}.plist")
    _stop_daemon(label)
    render_template(
        source / "templates/kanata-daemon.plist.in",
        plist,
        {
            "LABEL": label,
            "APP_BIN": app_bin,
            "CONFIG_PATH": config,
            "LOG_PATH": "/var/log/kanata.log",
        },
    )
    run(("chown", "root:wheel", plist))
    run(("launchctl", "bootstrap", "system", plist))
    run(("launchctl", "enable", f"system/{label}"))
    run(("launchctl", "kickstart", "-k", f"system/{label}"))
    return OperationResult(
        changed=True, msg="Reconciled the macOS Kanata launch daemon"
    )
