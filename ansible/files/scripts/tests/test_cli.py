from __future__ import annotations

from typer.main import get_command
from typer.testing import CliRunner

from workstation.automation import run_machine_protocol
from workstation.cli import app

runner = CliRunner()


def test_root_help_lists_command_groups() -> None:
    result = runner.invoke(app, ["--help"])

    assert result.exit_code == 0
    for group in ("apps", "chezmoi", "host", "macos", "hyper-window-tiling"):
        assert group in result.stdout


def test_nested_host_help_lists_python_replacements() -> None:
    result = runner.invoke(app, ["host", "--help"])

    assert result.exit_code == 0
    for group in ("apps", "desktop", "keyboard"):
        assert group in result.stdout


def test_version() -> None:
    result = runner.invoke(app, ["--version"])

    assert result.exit_code == 0
    assert result.stdout.strip() == "dotfiles-scripts version 0.1.0"


def _command_paths(
    command: object, prefix: tuple[str, ...] = ()
) -> list[tuple[str, ...]]:
    paths: list[tuple[str, ...]] = []
    commands = getattr(command, "commands", None)
    if isinstance(commands, dict):
        for name, child in commands.items():
            if not isinstance(name, str):
                continue
            path = (*prefix, name)
            paths.append(path)
            paths.extend(_command_paths(child, path))
    return paths


def test_every_grouped_command_renders_help() -> None:
    for path in _command_paths(get_command(app)):
        result = runner.invoke(app, [*path, "--help"])
        assert result.exit_code == 0, f"{' '.join(path)}: {result.output}"


def test_ansible_protocol_honors_check_mode_without_building() -> None:
    response = run_machine_protocol(
        '{"protocol":1,"command":["host","keyboard","kanata-build"],'
        '"context":{"repo_root":"/tmp/dotfiles","home":"/tmp/home"},'
        '"check":true,"diff":false}'
    )

    assert response.failed is False
    assert response.changed is True
    assert response.msg == "Would build a staged Kanata binary"


def test_ansible_protocol_rejects_unexposed_commands() -> None:
    response = run_machine_protocol(
        '{"protocol":1,"command":["chezmoi","shell-init"],'
        '"context":{"repo_root":"/tmp/dotfiles","home":"/tmp/home"},'
        '"check":true,"diff":false}'
    )

    assert response.failed is True
    assert response.msg is not None
    assert "not exposed to Ansible" in response.msg
