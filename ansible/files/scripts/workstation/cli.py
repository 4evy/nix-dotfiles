from __future__ import annotations

import typer

from workstation import __version__
from workstation.apps import app as apps_app
from workstation.automation import machine_entrypoint
from workstation.chezmoi import app as chezmoi_app
from workstation.console import error_console
from workstation.errors import DotfilesError
from workstation.host import app as host_app
from workstation.local import hyper_window_tiling_build
from workstation.macos import app as macos_app

app = typer.Typer(
    help="Repository, workstation, and host automation.",
    no_args_is_help=True,
    pretty_exceptions_enable=False,
)
app.add_typer(
    apps_app, name="apps", help="Install and configure workstation applications."
)
app.add_typer(chezmoi_app, name="chezmoi", help="Run Chezmoi lifecycle integrations.")
app.add_typer(
    hyper_window_tiling_build.app,
    name="hyper-window-tiling",
    help="Build the shared GNOME and KDE window-tiling package.",
)
app.add_typer(host_app, name="host", help="Configure the Linux host layer.")
app.add_typer(macos_app, name="macos", help="Configure macOS system integration.")
app.command("_ansible-v1", hidden=True)(machine_entrypoint)


def version_callback(value: bool) -> None:
    if value:
        typer.echo(f"dotfiles-scripts version {__version__}")
        raise typer.Exit()


@app.callback()
def main(
    version: bool = typer.Option(
        False,
        "--version",
        callback=version_callback,
        is_eager=True,
        help="Show the installed command version.",
    ),
) -> None:
    """Run dotfiles automation through the shared Python library."""


def entrypoint() -> None:
    try:
        app()
    except DotfilesError as error:
        error_console.print(f"[bold red]error:[/bold red] {error}")
        raise SystemExit(1) from error


if __name__ == "__main__":
    entrypoint()
