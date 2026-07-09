from __future__ import annotations


class DotfilesError(RuntimeError):
    """A user-facing automation failure."""


class UsageError(DotfilesError):
    """Invalid command-line usage outside the main Typer command tree."""
