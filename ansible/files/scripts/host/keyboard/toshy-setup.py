#!/usr/bin/env python3
"""Automate upstream Toshy's interactive installer for host Ansible runs."""

from __future__ import annotations

import argparse
import builtins
import os
import re
import runpy
import shlex
import shutil
import subprocess
import sys
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import TypeGuard, cast

RunCallable = Callable[..., subprocess.CompletedProcess[str]]
ORIGINAL_RUN = cast("RunCallable", subprocess.run)
SUDO_SHIM_DIR = os.environ.get("TOSHY_SUDO_SHIM_DIR")
SUDO_NAMES = {"sudo", "sudo-rs"}
SUDO_K_RE = re.compile(r"(^|[;&|]\s*)(?:/usr/bin/)?sudo(?:-rs)?\s+-k(?=\s*(?:[;&|]|$))")
SUDO_CMD_RE = re.compile(r"(^|[;&|]\s*)((?:/usr/bin/)?sudo(?:-rs)?)\s+")


def resolve_sudo() -> str:
    env_sudo = os.environ.get("TOSHY_SUDO")
    if env_sudo:
        return env_sudo

    for candidate in ("/run/wrappers/bin/sudo", "/usr/bin/sudo", "/bin/sudo"):
        if Path(candidate).is_file():
            return candidate

    for path_dir in os.environ.get("PATH", "").split(os.pathsep):
        if not path_dir:
            continue
        if SUDO_SHIM_DIR and Path(path_dir).resolve() == Path(SUDO_SHIM_DIR).resolve():
            continue
        candidate = Path(path_dir) / "sudo"
        if candidate.is_file():
            return str(candidate)

    return shutil.which("sudo") or "/usr/bin/sudo"


SUDO = resolve_sudo()


def is_sudo_command(command: str) -> bool:
    return Path(command).name in SUDO_NAMES


def rewrite_sudo_argv(argv: list[str]) -> list[str] | None:
    if not argv:
        return argv
    if not is_sudo_command(argv[0]):
        return argv
    rest = argv[1:]
    if rest == ["-k"]:
        return None
    if rest and rest[0] == "-n":
        rest = rest[1:]
    return [SUDO, "-n", *rest]


def rewrite_sudo_shell(command: str) -> str:
    sudo = shlex.quote(SUDO)
    command = SUDO_K_RE.sub(r"\1true", command)
    return SUDO_CMD_RE.sub(rf"\1{sudo} -n ", command)


def is_command_sequence(command: object) -> TypeGuard[Sequence[object]]:
    return isinstance(command, Sequence) and not isinstance(command, (str, bytes))


def automated_run(
    *popenargs: object, **kwargs: object
) -> subprocess.CompletedProcess[str]:
    if not popenargs:
        return ORIGINAL_RUN(*popenargs, **kwargs)

    command = popenargs[0]
    if kwargs.get("shell") and isinstance(command, str):
        popenargs = (rewrite_sudo_shell(command), *popenargs[1:])
    elif is_command_sequence(command):
        argv = [str(part) for part in command]
        rewritten = rewrite_sudo_argv(argv)
        if rewritten is None:
            return subprocess.CompletedProcess(argv, 0, "", "")
        popenargs = (rewritten, *popenargs[1:])

    return ORIGINAL_RUN(*popenargs, **kwargs)


def find_secret_code(prompt: str) -> str | None:
    secret_match = (
        re.search(r"secret code ['\"]([^'\"]+)['\"]", prompt, re.IGNORECASE)
        or re.search(
            r"secret code for this run is ['\"]([^'\"]+)['\"]", prompt, re.IGNORECASE
        )
        or re.search(
            r"enter the secret code ['\"]([^'\"]+)['\"]", prompt, re.IGNORECASE
        )
    )
    return secret_match.group(1) if secret_match else None


def answer_for(prompt: str) -> str:
    if secret_code := find_secret_code(prompt):
        return secret_code

    lowered = prompt.lower()
    if "have you updated your system recently" in lowered:
        return "y"
    if "run admin commands" in lowered:
        return "y"
    if "folder is not in path" in lowered:
        return "y"
    if "press enter to continue" in lowered:
        return ""
    if "install a kwin script" in lowered:
        return "n"
    if "barebones" in lowered and 'enter "yes" to proceed or "n" to exit' in lowered:
        return "YES"
    if 'enter "yes" to proceed or "n" to exit' in lowered:
        return "n"

    return ""


def automated_input(prompt: object = "") -> str:
    prompt_text = str(prompt)
    if prompt_text:
        print(prompt_text, end="", flush=True)
    response = answer_for(prompt_text)
    print(response, flush=True)
    return response


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("setup_path", type=Path)
    parser.add_argument("setup_args", nargs=argparse.REMAINDER)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    setup_path = args.setup_path.resolve()
    setup_dir = setup_path.parent

    os.chdir(setup_dir)
    sys.path.insert(0, str(setup_dir))
    builtins.input = automated_input  # ty: ignore[invalid-assignment]
    subprocess.run = automated_run  # ty: ignore[invalid-assignment]
    sys.argv = [str(setup_path), *args.setup_args]
    _ = runpy.run_path(str(setup_path), run_name="__main__")


if __name__ == "__main__":
    main()
