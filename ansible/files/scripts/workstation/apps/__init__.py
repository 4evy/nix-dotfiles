"""Application installer and launcher commands."""

from cyclopts import App

from workstation.apps import ghidra_mcp, installers

app = App(
    help="Install and configure workstation applications.",
    version_flags=[],
    result_action="return_none",
)
app.command(ghidra_mcp.install_ghidra_mcp, name="install-ghidra-mcp")
app.command(installers.install_ghostty_tip_linux, name="install-ghostty-tip-linux")
app.command(installers.install_helix_tip_linux, name="install-helix-tip-linux")
app.command(installers.install_helium_linux, name="install-helium-linux")
app.command(installers.install_helium_macos, name="install-helium-macos")
