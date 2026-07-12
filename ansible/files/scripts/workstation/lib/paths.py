import os
from pathlib import Path

from platformdirs import user_cache_path, user_data_path, user_state_path


def cache_path(*parts: str) -> Path:
    return user_cache_path("dotfiles").joinpath(*parts)


def data_path(*parts: str) -> Path:
    return user_data_path("dotfiles").joinpath(*parts)


def state_path(*parts: str) -> Path:
    return user_state_path("dotfiles").joinpath(*parts)


def state_path_for_home(home: Path, *parts: str) -> Path:
    configured = os.environ.get("XDG_STATE_HOME")
    root = Path(configured).expanduser() if configured else home / ".local/state"
    return root.joinpath("dotfiles", *parts)


def find_repo_root(start: str | Path) -> Path:
    current = Path(start).expanduser().resolve()
    if current.is_file():
        current = current.parent
    for candidate in (current, *current.parents):
        if (candidate / "pyproject.toml").is_file() and (
            candidate / "ansible"
        ).is_dir():
            return candidate
    raise FileNotFoundError(f"could not find dotfiles repository root from {start}")
