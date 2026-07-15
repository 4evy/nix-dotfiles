from pathlib import Path
from typing import override

from spectrum_build.core.common import require_readable_file
from spectrum_build.core.context import BuildContext
from spectrum_build.integrations.repositories import RepositoryFile
from spectrum_build.programs.models import DnfProgram, SystemGroup

LOCAL_PREFIX = Path("/var/usrlocal")
MCP_BINARY = Path("/opt/1Password/1password-mcp")
MCP_COMMAND = Path("/usr/bin/1password-mcp")


def _prepare_mcp_command(local_prefix: Path = LOCAL_PREFIX) -> None:
    # /usr/local points here, but bootc creates the target only on deployed hosts.
    (local_prefix / "bin").mkdir(parents=True, exist_ok=True)


def _relocate_mcp_command(
    local_prefix: Path = LOCAL_PREFIX,
    binary: Path = MCP_BINARY,
    command: Path = MCP_COMMAND,
) -> None:
    require_readable_file(binary)
    local_bin = local_prefix / "bin"
    (local_bin / "1password-mcp").unlink(missing_ok=True)
    local_bin.rmdir()
    local_prefix.rmdir()
    command.unlink(missing_ok=True)
    command.symlink_to(binary)


class OnePasswordProgram(DnfProgram):
    @override
    def install(self, context: BuildContext) -> None:
        _prepare_mcp_command()
        super().install(context)
        _relocate_mcp_command()


PROGRAM = OnePasswordProgram(
    name="1Password",
    packages=("1password", "1password-cli"),
    repositories=(
        RepositoryFile(
            destination=Path("/etc/pki/rpm-gpg/RPM-GPG-KEY-1password"),
            source="https://downloads.1password.com/linux/keys/1password.asc",
            import_rpm_key=True,
        ),
        RepositoryFile(
            destination=Path("/etc/yum.repos.d/1password.repo"),
            source=Path("image/repos/1password.repo"),
            repo_ids=("1password",),
        ),
    ),
    enabled_repositories=("1password",),
    system_groups=(
        SystemGroup("onepassword-mcp", 954),
        SystemGroup("onepassword-cli", 955),
        SystemGroup("onepassword", 956),
    ),
    validation_packages=("1password", "1password-cli"),
)
