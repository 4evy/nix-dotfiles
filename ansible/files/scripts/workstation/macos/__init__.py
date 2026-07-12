"""macOS workstation configuration commands."""

import typer

from workstation.macos import system

app = typer.Typer(no_args_is_help=True, pretty_exceptions_enable=False)
app.command("karabiner-vhid")(system.configure_karabiner_vhid)
app.command("kanata")(system.configure_kanata)
