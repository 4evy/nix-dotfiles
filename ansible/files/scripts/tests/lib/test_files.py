import io
import tarfile
from pathlib import Path

from workstation.lib.files import extract_tar_archive


def test_extract_tar_archive_extracts_regular_files(tmp_path: Path) -> None:
    archive_path = tmp_path / "example.tar"
    content = b"hello from the archive\n"
    with tarfile.open(archive_path, "w") as archive:
        member = tarfile.TarInfo("nested/example.txt")
        member.size = len(content)
        archive.addfile(member, io.BytesIO(content))

    destination = tmp_path / "destination"
    with tarfile.open(archive_path) as archive:
        extract_tar_archive(archive, destination)

    assert (destination / "nested/example.txt").read_bytes() == content
