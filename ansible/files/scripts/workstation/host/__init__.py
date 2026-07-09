"""Linux host-layer automation commands."""

from __future__ import annotations

import typer

from workstation.host import apps, desktop, keyboard

app = typer.Typer(no_args_is_help=True, pretty_exceptions_enable=False)
app.add_typer(
    apps.app, name="apps", help="Configure host networking and remote desktop."
)
app.add_typer(desktop.app, name="desktop", help="Configure Linux desktop integrations.")
app.add_typer(keyboard.app, name="keyboard", help="Configure Kanata and Toshy.")
