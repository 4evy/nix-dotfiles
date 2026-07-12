import json
import time
from pathlib import Path
from typing import TYPE_CHECKING

from workstation.apps import installers
from workstation.automation import run_machine_protocol

if TYPE_CHECKING:
    import pytest


def _payload(tmp_path: Path, command: list[str], *, check: bool = True) -> str:
    return json.dumps({
        "protocol": 1,
        "command": command,
        "context": {
            "repo_root": str(tmp_path),
            "home": str(tmp_path / "home"),
            "cache_dir": str(tmp_path / "cache"),
            "data_dir": str(tmp_path / "data"),
            "bin_dir": str(tmp_path / "bin"),
            "system": "Linux",
            "architecture": "x86_64",
        },
        "check": check,
        "diff": True,
    })


def test_machine_protocol_uses_typer_and_operation_result(tmp_path: Path) -> None:
    response = run_machine_protocol(
        _payload(tmp_path, ["host", "keyboard", "kanata-build"])
    )

    assert not response.failed
    assert response.changed
    assert response.msg == "Would build a staged Kanata binary"
    assert response.data["executable"] == str(
        tmp_path / "data/host/bin-staging/kanata/root/bin/kanata"
    )


def test_machine_protocol_skips_maintenance_in_check_mode(tmp_path: Path) -> None:
    response = run_machine_protocol(
        _payload(tmp_path, ["host", "desktop", "flatpak-maintenance"])
    )

    assert not response.failed
    assert not response.changed
    assert response.skipped


def test_machine_protocol_rejects_commands_outside_allowlist(tmp_path: Path) -> None:
    response = run_machine_protocol(_payload(tmp_path, ["chezmoi", "shell-init"]))

    assert response.failed
    assert response.msg is not None
    assert "not exposed to Ansible automation" in response.msg


def test_machine_protocol_rejects_unknown_context(tmp_path: Path) -> None:
    payload = json.loads(_payload(tmp_path, ["host", "desktop", "flatpak-maintenance"]))
    payload["context"]["hostvars"] = {"secret": "must not cross the boundary"}

    response = run_machine_protocol(json.dumps(payload))

    assert response.failed
    assert response.msg is not None
    assert "Extra inputs are not permitted" in response.msg


def test_ghostty_check_mode_does_not_create_directories(tmp_path: Path) -> None:
    payload = _payload(
        tmp_path,
        [
            "apps",
            "install-ghostty-tip-linux",
            str(tmp_path / "cache"),
            str(tmp_path / "prefix"),
        ],
    )

    response = run_machine_protocol(payload)

    assert not response.failed
    assert response.changed
    assert not (tmp_path / "cache").exists()
    assert not (tmp_path / "prefix").exists()


def test_ghostty_current_state_is_idempotent(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(installers, "require_commands", lambda *_args: None)
    monkeypatch.setattr(installers, "_missing_libraries", lambda _path: [])
    prefix = tmp_path / "prefix"
    executable = prefix / "bin/ghostty"
    executable.parent.mkdir(parents=True)
    executable.write_text("#!/bin/sh\n")
    executable.chmod(0o755)
    (prefix / ".ghostty-tip-checked-at").write_text(f"{int(time.time())}\n")
    (prefix / ".ghostty-tip-source-key").write_text("current\n")
    (prefix / ".ghostty-tip-state-version").write_text("1\n")
    payload = _payload(
        tmp_path,
        [
            "apps",
            "install-ghostty-tip-linux",
            str(tmp_path / "cache"),
            str(prefix),
        ],
        check=False,
    )

    response = run_machine_protocol(payload)

    assert not response.failed
    assert not response.changed
    assert response.msg == "Ghostty tip was checked recently"
