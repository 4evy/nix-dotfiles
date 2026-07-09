from __future__ import annotations

import sys

import pytest

from workstation.local.jj import jj_get_entrypoint, jj_redate_entrypoint


def test_jj_get_help_does_not_require_a_repository(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr(sys, "argv", ["jj-get", "--help"])

    jj_get_entrypoint()

    assert capsys.readouterr().out.startswith("usage: jj-get")


@pytest.mark.parametrize(
    "arguments",
    [
        ["123", "owner/repo", "ignored"],
        ["https://github.com/owner/repo/pull/123", "ignored"],
    ],
)
def test_jj_get_rejects_extra_pr_arguments(
    monkeypatch: pytest.MonkeyPatch, arguments: list[str]
) -> None:
    monkeypatch.setattr(sys, "argv", ["jj-get", *arguments])

    with pytest.raises(SystemExit, match="usage: jj-get"):
        jj_get_entrypoint()


def test_jj_redate_help_does_not_prompt(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr(sys, "argv", ["jj-redate", "--help"])

    jj_redate_entrypoint()

    assert capsys.readouterr().out.startswith("usage: jj-redate")
