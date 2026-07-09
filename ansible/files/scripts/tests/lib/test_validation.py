from __future__ import annotations

from pathlib import Path

import pytest

from workstation.errors import DotfilesError
from workstation.lib.validation import (
    octal_mode,
    path_component,
    safe_path,
    template_name,
)


@pytest.mark.parametrize("value", ["", ".", "..", "nested/name", "/absolute"])
def test_path_component_rejects_traversal_and_nested_paths(value: str) -> None:
    with pytest.raises(DotfilesError, match="single path component"):
        path_component(value)


@pytest.mark.parametrize("value", ["", ".", "..", "/", "//", "nested/.."])
def test_safe_path_rejects_destructive_targets(value: str) -> None:
    with pytest.raises(DotfilesError, match="unsafe path"):
        safe_path(value)


def test_safe_path_preserves_a_normal_relative_path() -> None:
    assert safe_path("nested/value") == Path("nested/value")


@pytest.mark.parametrize("value", ["", "8", "0o755", "-1", "755 "])
def test_octal_mode_rejects_non_octal_input(value: str) -> None:
    with pytest.raises(DotfilesError, match="must be octal"):
        octal_mode(value)


@pytest.mark.parametrize("value", ["NAME", "NAME_2", "A"])
def test_template_name_accepts_environment_style_names(value: str) -> None:
    assert template_name(value) == value


@pytest.mark.parametrize("value", ["", "lower", "2FAST", "HAS-DASH", "HAS SPACE"])
def test_template_name_rejects_ambiguous_replacements(value: str) -> None:
    with pytest.raises(DotfilesError, match="invalid template replacement name"):
        template_name(value)
