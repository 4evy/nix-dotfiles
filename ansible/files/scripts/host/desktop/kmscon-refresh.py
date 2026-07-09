#!/usr/bin/env python3
from __future__ import annotations

import subprocess


def main() -> int:
    sessions = subprocess.run(
        ("/usr/bin/loginctl", "list-sessions", "--no-legend"),
        check=False,
        capture_output=True,
        text=True,
    ).stdout.splitlines()
    active_ttys = {fields[4] for line in sessions if len(fields := line.split()) >= 5}
    for number in range(1, 7):
        tty = f"tty{number}"
        if tty not in active_ttys:
            subprocess.run(
                ("/usr/bin/systemctl", "try-restart", f"kmsconvt@{tty}.service"),
                check=False,
            )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
