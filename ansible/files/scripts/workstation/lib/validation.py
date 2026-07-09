from __future__ import annotations

import re
import string
from pathlib import Path

from workstation.errors import DotfilesError

_TEMPLATE_NAME = re.compile(r"^[A-Z][A-Z0-9_]*$")


def path_component(value: str, *, label: str = "path component") -> str:
    if not value or value in {".", ".."} or "/" in value:
        raise DotfilesError(f"{label} must be a single path component: {value!r}")
    return value


def safe_path(value: str | Path) -> Path:
    path = Path(value)
    if str(path) in {"", "/", "//", ".", ".."} or path.name in {".", ".."}:
        raise DotfilesError(f"refusing unsafe path: {path}")
    return path


def octal_mode(value: str | int, *, label: str = "mode") -> int:
    text = str(value)
    if not text or any(character not in string.octdigits for character in text):
        raise DotfilesError(f"{label} must be octal: {value}")
    return int(text, 8)


def template_name(value: str) -> str:
    if not _TEMPLATE_NAME.fullmatch(value):
        raise DotfilesError(f"invalid template replacement name: {value}")
    return value
