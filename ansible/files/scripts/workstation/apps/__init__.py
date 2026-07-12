"""Application installer and launcher commands."""

import typer

from workstation.apps import ghidra_mcp, installers

app = typer.Typer(no_args_is_help=True, pretty_exceptions_enable=False)
app.command("install-ghidra-mcp")(ghidra_mcp.install_ghidra_mcp)
app.command("install-ghostty-tip-linux")(installers.install_ghostty_tip_linux)
app.command("install-helix-tip-linux")(installers.install_helix_tip_linux)
app.command("install-helium-linux")(installers.install_helium_linux)
app.command("install-helium-macos")(installers.install_helium_macos)
