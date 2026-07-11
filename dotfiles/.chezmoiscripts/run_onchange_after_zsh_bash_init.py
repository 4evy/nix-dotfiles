#!/usr/bin/env python3
from __future__ import annotations

import os
import shutil
from pathlib import Path

command = Path.home() / ".local/bin/dotfiles-scripts"
if not command.is_file():
    command = Path(shutil.which("dotfiles-scripts") or command)
if command.is_file() and os.access(command, os.X_OK):
    os.execv(command, (command, "chezmoi", "shell-init"))
