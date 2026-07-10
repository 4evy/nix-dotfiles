from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import pytest

from workstation.local import jj as jj_module
from workstation.local.jj import jj_get_entrypoint, jj_redate_entrypoint


class Tty:
    def isatty(self) -> bool:
        return True


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


def test_jj_redate_gum_input_keeps_prompt_on_terminal(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[tuple[str, ...], dict[str, object]]] = []

    def fake_run(
        argv: tuple[str, ...],
        **kwargs: object,
    ) -> subprocess.CompletedProcess[str]:
        calls.append((argv, kwargs))
        return subprocess.CompletedProcess(argv, 0, stdout="2026-07-10\n")

    monkeypatch.delenv("JJ_REDATE_NO_GUM", raising=False)
    monkeypatch.setattr(sys, "stdin", Tty())
    monkeypatch.setattr(sys, "stdout", Tty())
    monkeypatch.setattr(
        jj_module,
        "which",
        lambda name: Path("/opt/homebrew/bin/gum") if name == "gum" else None,
    )
    monkeypatch.setattr(jj_module.subprocess, "run", fake_run)

    assert jj_module._prompt("Date (YYYY-MM-DD): ", "2026-07-10") == "2026-07-10"
    assert calls == [
        (
            (
                "/opt/homebrew/bin/gum",
                "input",
                "--prompt",
                "Date (YYYY-MM-DD): ",
                "--value",
                "2026-07-10",
            ),
            {"check": False, "stdout": subprocess.PIPE, "text": True},
        )
    ]


def test_jj_redate_gum_confirm_is_interactive(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[tuple[str, ...], dict[str, object]]] = []

    def fake_run(
        argv: tuple[str, ...],
        **kwargs: object,
    ) -> subprocess.CompletedProcess[str]:
        calls.append((argv, kwargs))
        return subprocess.CompletedProcess(argv, 0)

    monkeypatch.delenv("JJ_REDATE_NO_GUM", raising=False)
    monkeypatch.setattr(sys, "stdin", Tty())
    monkeypatch.setattr(sys, "stdout", Tty())
    monkeypatch.setattr(
        jj_module,
        "which",
        lambda name: Path("/opt/homebrew/bin/gum") if name == "gum" else None,
    )
    monkeypatch.setattr(jj_module.subprocess, "run", fake_run)

    assert jj_module._confirm_redate(["@-"], "2026-07-10T03:25:00+03:00")
    assert calls == [
        (
            (
                "/opt/homebrew/bin/gum",
                "confirm",
                (
                    "Set author and committer timestamp on @- to "
                    "2026-07-10T03:25:00+03:00?"
                ),
            ),
            {"check": False},
        )
    ]


def test_jj_redate_without_args_falls_back_to_working_copy(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("JJ_REDATE_NO_GUM", "1")
    monkeypatch.setattr(sys, "stdin", Tty())
    monkeypatch.setattr(sys, "stdout", Tty())

    assert jj_module._redate_revisions([]) == ["@"]


def test_jj_redate_without_args_opens_interactive_revision_picker(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[tuple[str, ...], dict[str, object]]] = []

    def fake_log(revset: str, template: str, reverse: bool = False) -> str:
        assert not reverse
        assert "mutable() & remote_bookmarks().." in revset
        assert "change_id" in template
        return (
            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\t@\taaaaaaaa\tuser@example.com\t"
            "2026-01-02 03:04:05\t11111111\t\n"
            "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\to\tbbbbbbbb\tuser@example.com\t"
            "2026-01-02 03:00:00\t22222222\tadd sample feature\n"
        )

    def fake_run(
        argv: tuple[str, ...],
        **kwargs: object,
    ) -> subprocess.CompletedProcess[str]:
        calls.append((argv, kwargs))
        return subprocess.CompletedProcess(
            argv,
            0,
            stdout=(
                "o bbbbbbbb user@example.com 2026-01-02 03:00:00 "
                "22222222  add sample feature\n"
            ),
        )

    monkeypatch.delenv("JJ_REDATE_NO_GUM", raising=False)
    monkeypatch.delenv("JJ_REDATE_REVSET", raising=False)
    monkeypatch.delenv("JJ_REDATE_LIMIT", raising=False)
    monkeypatch.setattr(sys, "stdin", Tty())
    monkeypatch.setattr(sys, "stdout", Tty())
    monkeypatch.setattr(
        jj_module,
        "which",
        lambda name: Path("/opt/homebrew/bin/gum") if name == "gum" else None,
    )
    monkeypatch.setattr(jj_module, "_log", fake_log)
    monkeypatch.setattr(jj_module.subprocess, "run", fake_run)

    assert jj_module._redate_revisions([]) == [
        "change_id(bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb)"
    ]
    assert calls == [
        (
            (
                "/opt/homebrew/bin/gum",
                "choose",
                "--ordered",
                "--limit",
                "2",
                "--height",
                "5",
                "--header",
                "Select revisions to redate",
                (
                    "@ aaaaaaaa user@example.com 2026-01-02 03:04:05 "
                    "11111111  (no description set)"
                ),
                (
                    "o bbbbbbbb user@example.com 2026-01-02 03:00:00 "
                    "22222222  add sample feature"
                ),
            ),
            {"check": False, "stdout": subprocess.PIPE, "text": True},
        )
    ]
