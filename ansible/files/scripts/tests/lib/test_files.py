from pathlib import Path

from workstation.lib.files import (
    fresh_directory,
    remove_path,
    replace_directory,
)


def test_remove_path_handles_files_links_and_directories(tmp_path: Path) -> None:
    target = tmp_path / "target"
    target.mkdir()
    (target / "nested").write_text("data")
    link = tmp_path / "link"
    link.symlink_to(target, target_is_directory=True)
    file = tmp_path / "file"
    file.write_text("data")

    remove_path(link)
    remove_path(file)
    remove_path(target)
    remove_path(tmp_path / "missing")

    assert not link.exists()
    assert not file.exists()
    assert not target.exists()


def test_fresh_directory_replaces_a_symlink(tmp_path: Path) -> None:
    target = tmp_path / "target"
    target.mkdir()
    link = tmp_path / "link"
    link.symlink_to(target, target_is_directory=True)

    assert fresh_directory(link) == link
    assert link.is_dir()
    assert not link.is_symlink()
    assert target.is_dir()


def test_replace_directory_preserves_metadata_and_symlinks(tmp_path: Path) -> None:
    source = tmp_path / "source"
    source.mkdir()
    executable = source / "executable"
    executable.write_text("new")
    executable.chmod(0o751)
    (source / "link").symlink_to("executable")
    destination = tmp_path / "destination"
    destination.mkdir()
    (destination / "stale").write_text("old")

    replace_directory(source, destination)

    assert (destination / "executable").read_text() == "new"
    assert (destination / "executable").stat().st_mode & 0o777 == 0o751
    assert (destination / "link").is_symlink()
    assert (destination / "link").readlink() == Path("executable")
    assert not (destination / "stale").exists()
