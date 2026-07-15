"""Linux host-layer automation commands."""

from cyclopts import App

from workstation.host import apps, desktop, keyboard

app = App(
    help="Configure the Linux host layer.",
    version_flags=[],
    result_action="return_none",
)
app.command(apps.app, name="apps")
app.command(desktop.app, name="desktop")
app.command(keyboard.app, name="keyboard")
