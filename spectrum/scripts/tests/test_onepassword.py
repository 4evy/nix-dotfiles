from pathlib import Path

from spectrum_build.programs.onepassword import (
    _prepare_mcp_command,
    _relocate_mcp_command,
)


def test_onepassword_mcp_command_is_relocated_out_of_mutable_usr_local(
    tmp_path: Path,
) -> None:
    local_prefix = tmp_path / "var/usrlocal"
    binary = tmp_path / "opt/1Password/1password-mcp"
    command = tmp_path / "usr/bin/1password-mcp"
    binary.parent.mkdir(parents=True)
    binary.write_text("binary")
    command.parent.mkdir(parents=True)

    _prepare_mcp_command(local_prefix)
    (local_prefix / "bin/1password-mcp").symlink_to(binary)
    _relocate_mcp_command(local_prefix, binary, command)

    assert not local_prefix.exists()
    assert command.is_symlink()
    assert command.resolve() == binary
