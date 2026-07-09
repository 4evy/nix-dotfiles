from __future__ import annotations

import filecmp
import os
import shutil
import stat
import tarfile
import tempfile
from pathlib import Path

from boltons.fileutils import AtomicSaver

from workstation.errors import DotfilesError
from workstation.lib.validation import octal_mode, safe_path


def require_file(path: str | Path) -> Path:
    result = Path(path)
    if not result.is_file():
        raise DotfilesError(f"required file does not exist: {result}")
    return result


def require_directory(path: str | Path) -> Path:
    result = Path(path)
    if not result.is_dir():
        raise DotfilesError(f"required directory does not exist: {result}")
    return result


def require_executable(path: str | Path) -> Path:
    result = Path(path)
    if not result.is_file() or not os.access(result, os.X_OK):
        raise DotfilesError(
            f"required executable does not exist or is not executable: {result}"
        )
    return result


def ensure_directory(path: str | Path, mode: int | str | None = None) -> Path:
    result = safe_path(path)
    result.mkdir(parents=True, exist_ok=True)
    if mode is not None:
        result.chmod(octal_mode(mode, label="directory mode"))
    return result


def fresh_directory(path: str | Path, mode: int | str | None = None) -> Path:
    result = safe_path(path)
    if result.is_symlink() or result.is_file():
        result.unlink()
    elif result.exists():
        shutil.rmtree(result)
    return ensure_directory(result, mode)


def extract_tar_archive(archive: tarfile.TarFile, destination: str | Path) -> None:
    """Extract an archive using Python's hardened data filter on every runtime."""
    destination_path = ensure_directory(destination)
    data_filter = getattr(tarfile, "data_filter", None)
    if data_filter is not None:
        try:
            archive.extractall(destination_path, filter="data")
            return
        except TypeError:
            members = [
                filtered
                for member in archive.getmembers()
                if (filtered := data_filter(member, destination_path)) is not None
            ]
            archive.extractall(destination_path, members=members)
            return

    root = destination_path.resolve()
    members: list[tarfile.TarInfo] = []
    for member in archive.getmembers():
        target = (root / member.name).resolve()
        if target != root and root not in target.parents:
            raise DotfilesError(
                f"tar member escapes extraction directory: {member.name}"
            )
        if member.islnk() or member.issym():
            link = (target.parent / member.linkname).resolve()
            if link != root and root not in link.parents:
                raise DotfilesError(
                    f"tar link escapes extraction directory: {member.name}"
                )
        members.append(member)
    archive.extractall(destination_path, members=members)


def install_file_if_changed(
    source: str | Path,
    destination: str | Path,
    mode: int | str = "0644",
) -> bool:
    source_path = require_file(source)
    destination_path = safe_path(destination)
    parsed_mode = octal_mode(mode, label="file mode")
    content_matches = destination_path.is_file() and filecmp.cmp(
        source_path, destination_path, shallow=False
    )
    mode_matches = (
        content_matches and stat.S_IMODE(destination_path.stat().st_mode) == parsed_mode
    )
    if content_matches and mode_matches:
        return False

    destination_path.parent.mkdir(parents=True, exist_ok=True)
    with (
        AtomicSaver(
            str(destination_path),
            overwrite=True,
            text_mode=False,
            file_perms=parsed_mode,
        ) as target,
        source_path.open("rb") as source_file,
    ):
        shutil.copyfileobj(source_file, target)
    destination_path.chmod(parsed_mode)
    return True


def write_if_changed(
    destination: str | Path,
    content: str | bytes,
    mode: int | str = "0644",
) -> bool:
    destination_path = safe_path(destination)
    data = content.encode() if isinstance(content, str) else content
    parsed_mode = octal_mode(mode, label="file mode")
    if destination_path.is_file():
        current_mode = stat.S_IMODE(destination_path.stat().st_mode)
        if destination_path.read_bytes() == data and current_mode == parsed_mode:
            return False

    destination_path.parent.mkdir(parents=True, exist_ok=True)
    with AtomicSaver(
        str(destination_path),
        overwrite=True,
        text_mode=False,
        file_perms=parsed_mode,
    ) as target:
        target.write(data)
    destination_path.chmod(parsed_mode)
    return True


def replace_directory(source: str | Path, destination: str | Path) -> None:
    source_path = require_directory(source)
    destination_path = safe_path(destination)
    destination_path.parent.mkdir(parents=True, exist_ok=True)
    temporary_path = Path(
        tempfile.mkdtemp(
            prefix=f".{destination_path.name}-", dir=destination_path.parent
        )
    )
    try:
        shutil.copytree(source_path, temporary_path, dirs_exist_ok=True, symlinks=True)
        if destination_path.is_symlink() or destination_path.is_file():
            destination_path.unlink()
        elif destination_path.exists():
            shutil.rmtree(destination_path)
        temporary_path.replace(destination_path)
    finally:
        if temporary_path.exists():
            shutil.rmtree(temporary_path)
