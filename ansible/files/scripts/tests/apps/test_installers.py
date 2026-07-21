import json
import os
from collections.abc import Sequence
from pathlib import Path
from typing import TYPE_CHECKING

from workstation.apps import installers
from workstation.lib.commands import CommandResult

if TYPE_CHECKING:
    import pytest


def test_helium_input_is_prepared_outside_go(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    commands: list[tuple[str, ...]] = []

    def fake_run(
        argv: Sequence[str | os.PathLike[str]], **_kwargs: object
    ) -> CommandResult:
        command = tuple(map(str, argv))
        commands.append(command)
        if command[0] == "sops":
            return CommandResult(0, '["[*.]example.com"]\n', "")
        return CommandResult(0, "caller-supplied-token\n", "")

    monkeypatch.setattr(installers, "run", fake_run)
    monkeypatch.setattr(installers, "which", lambda _name: tmp_path / "gh")

    apply_input = json.loads(installers._helium_apply_input(tmp_path / "secrets.yaml"))

    assert apply_input == {
        "cookie_allowlist": ["[*.]example.com"],
        "extension_values": {"refined-github-personal-token": "caller-supplied-token"},
    }
    assert commands[0][0] == "sops"
    assert commands[1] == ("gh", "auth", "token")


def test_helium_input_omits_unavailable_private_values(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(
        installers,
        "run",
        lambda *_args, **_kwargs: CommandResult(1, "", "unavailable"),
    )
    monkeypatch.setattr(installers, "which", lambda _name: None)

    assert json.loads(installers._helium_apply_input(tmp_path / "secrets.yaml")) == {}
