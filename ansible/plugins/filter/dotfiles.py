"""Controller-side filters for declarative dotfiles state."""

from __future__ import annotations

import ast
from collections.abc import Callable


def merge_gvariant_string_list(current: object, value: str) -> str:
    """Append one string to a GVariant string list without losing existing values."""
    text = current if isinstance(current, str) else ""
    text = text.strip().removeprefix("@as ").strip()
    try:
        parsed = ast.literal_eval(text)
    except (SyntaxError, ValueError):  # fmt: skip
        parsed = []
    if not isinstance(parsed, list):
        parsed = []
    result = [item for item in parsed if isinstance(item, str)]
    if value not in result:
        result.append(value)
    return repr(result)


class FilterModule:
    """Expose dotfiles filters to Ansible/Jinja."""

    def filters(self) -> dict[str, Callable[..., object]]:
        """Return filters exported by this plugin."""
        return {"merge_gvariant_string_list": merge_gvariant_string_list}
