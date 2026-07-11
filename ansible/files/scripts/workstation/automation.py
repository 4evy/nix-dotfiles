"""Versioned machine interface for Ansible-owned Python operations."""

from __future__ import annotations

import json
import sys
from collections.abc import Iterator
from contextlib import contextmanager, redirect_stdout
from contextvars import ContextVar

from pydantic import ValidationError
from typer.main import get_command

from workstation.automation_models import (
    AutomationContext,
    OperationRequest,
    OperationResponse,
    OperationResult,
)
from workstation.errors import DotfilesError

_REQUEST: ContextVar[OperationRequest | None] = ContextVar(
    "dotfiles_automation_request", default=None
)

_ALLOWED_COMMANDS = {
    ("apps", "install-ghostty-tip-linux"),
    ("apps", "install-helium-linux"),
    ("apps", "install-helium-macos"),
    ("apps", "install-helix-tip-linux"),
    ("host", "apps", "rustdesk-tailscale"),
    ("host", "apps", "tailscale-bluefin"),
    ("host", "desktop", "flatpak-maintenance"),
    ("host", "desktop", "flatpak-nvidia"),
    ("host", "desktop", "hyper-window-tiling"),
    ("host", "desktop", "sushi-preview"),
    ("host", "keyboard", "kanata-build"),
    ("host", "keyboard", "toshy-gnome-context"),
    ("host", "keyboard", "toshy-kanata-chain"),
    ("host", "keyboard", "toshy-kanata-check"),
    ("macos", "kanata"),
    ("macos", "karabiner-vhid"),
}


def current_request() -> OperationRequest | None:
    """Return the active machine request, if the function was called by Ansible."""
    return _REQUEST.get()


def current_context() -> AutomationContext | None:
    """Return the selected Ansible context for the active operation."""
    request = current_request()
    return request.context if request is not None else None


def automation_check_mode() -> bool:
    """Whether the active machine request is a no-write check-mode invocation."""
    request = current_request()
    return request.check if request is not None else False


@contextmanager
def _request_scope(request: OperationRequest) -> Iterator[None]:
    token = _REQUEST.set(request)
    try:
        yield
    finally:
        _REQUEST.reset(token)


def _require_allowed_command(command: list[str]) -> None:
    if not any(tuple(command[: len(prefix)]) == prefix for prefix in _ALLOWED_COMMANDS):
        raise DotfilesError(
            f"command is not exposed to Ansible automation: {' '.join(command)}"
        )


def dispatch(request: OperationRequest) -> OperationResponse:
    """Execute a command through Typer's existing parser and normalize its result."""
    if request.command[0] == "_ansible-v1":
        raise DotfilesError("the machine endpoint cannot invoke itself")
    _require_allowed_command(request.command)
    command = " ".join(request.command)
    from workstation.cli import app

    with _request_scope(request):
        result = get_command(app).main(
            args=request.command,
            prog_name="dotfiles-scripts",
            standalone_mode=False,
        )
    if not isinstance(result, OperationResult):
        raise DotfilesError(
            f"automation command did not return OperationResult: {command}"
        )
    return OperationResponse(**result.model_dump())


def _failure(message: str) -> OperationResponse:
    return OperationResponse(failed=True, msg=message)


def run_machine_protocol(payload: str) -> OperationResponse:
    """Execute one JSON payload without leaking exceptions into the wire format."""
    try:
        request = OperationRequest.model_validate_json(payload)
        return dispatch(request)
    except (DotfilesError, ValidationError, json.JSONDecodeError) as error:
        return _failure(str(error))
    except Exception as error:  # noqa: BLE001 - wire boundary must stay valid JSON.
        return _failure(f"unexpected automation failure: {error}")


def machine_entrypoint() -> None:
    """Read one request from stdin and emit exactly one response on stdout."""
    output = sys.stdout
    payload = sys.stdin.read()
    with redirect_stdout(sys.stderr):
        response = run_machine_protocol(payload)
    output.write(response.model_dump_json(exclude_none=True) + "\n")
