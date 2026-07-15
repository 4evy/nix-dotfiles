"""Linux host-layer automation commands."""

from cyclopts import App

from workstation.host import apps

app = App(
    help="Configure the Linux host layer.",
    version_flags=[],
    result_action="return_none",
)
app.command(apps.app, name="apps")
