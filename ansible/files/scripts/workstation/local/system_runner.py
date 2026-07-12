import os
import shutil
import sys
from collections.abc import Sequence
from dataclasses import dataclass
from pathlib import Path

from workstation.errors import UsageError

DEFAULT_RUNNER_PATH = ":".join((
    "/usr/sbin",
    "/usr/bin",
    "/sbin",
    "/bin",
    "/usr/local/sbin",
    "/usr/local/bin",
    "/home/linuxbrew/.linuxbrew/bin",
    "/home/linuxbrew/.linuxbrew/sbin",
    "/opt/homebrew/bin",
    "/opt/homebrew/sbin",
    "/run/wrappers/bin",
    "/run/current-system/sw/bin",
    "/nix/var/nix/profiles/default/bin",
    "/etc/profiles/per-user/root/bin",
))


@dataclass(frozen=True, slots=True)
class Invocation:
    environment: dict[str, str]
    program: str
    arguments: tuple[str, ...]


def _parse_environment(value: str) -> tuple[str, str]:
    if "=" not in value:
        raise UsageError("--env requires KEY=VALUE")
    key, setting = value.split("=", 1)
    if not key:
        raise UsageError("--env variable name must not be empty")
    if "=" in key or "\0" in key:
        raise UsageError(f"invalid environment variable name: {key!r}")
    return key, setting


def parse_invocation(argv: Sequence[str]) -> Invocation | None:
    overrides: dict[str, str] = {}
    index = 0
    while index < len(argv):
        argument = argv[index]
        if argument == "--version":
            return None
        if argument == "--":
            index += 1
            break
        if argument == "--env":
            index += 1
            if index == len(argv):
                raise UsageError("--env requires KEY=VALUE")
            key, value = _parse_environment(argv[index])
            overrides[key] = value
            index += 1
            continue
        if argument.startswith("--env="):
            key, value = _parse_environment(argument.removeprefix("--env="))
            overrides[key] = value
            index += 1
            continue
        if argument.startswith("-"):
            raise UsageError(f"unknown flag: {argument}")
        break

    command = tuple(argv[index:])
    if not command:
        raise UsageError("expected COMMAND [ARG...]")

    path_value = overrides.get("PATH", DEFAULT_RUNNER_PATH)
    program, *arguments = command
    if "/" in program:
        candidate = Path(program)
        if (
            candidate.exists()
            and candidate.is_file()
            and not os.access(candidate, os.X_OK)
        ):
            arguments.insert(0, program)
            program = "/bin/cat"
    else:
        resolved = shutil.which(program, path=path_value)
        if resolved is not None:
            program = resolved

    environment = dict(os.environ)
    environment.update(overrides)
    environment["PATH"] = path_value
    return Invocation(environment, program, tuple(arguments))


def main(argv: Sequence[str] | None = None) -> int:
    arguments = tuple(sys.argv[1:] if argv is None else argv)
    try:
        invocation = parse_invocation(arguments)
    except UsageError as error:
        print(f"system-runner: {error}", file=sys.stderr)
        return 2
    if invocation is None:
        print("system-runner version dev")
        return 0
    os.execvpe(
        invocation.program,
        (invocation.program, *invocation.arguments),
        invocation.environment,
    )
    return 127


def entrypoint() -> None:
    try:
        raise SystemExit(main())
    except OSError as error:
        raise SystemExit(f"system-runner: {error}") from error


if __name__ == "__main__":
    entrypoint()
