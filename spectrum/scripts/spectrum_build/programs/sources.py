import json
import platform
import sys
from pathlib import Path
from typing import TypedDict

from spectrum_build.core.context import BuildContext
from spectrum_build.image.platform_info import fedora_arch
from spectrum_build.integrations.github import ReleaseRpm
from spectrum_build.programs.models import PackageResolver


class SourcePin(TypedDict):
    revision: str
    version: str


class GhosttySourcePin(SourcePin):
    source_sha256: str
    zig_sha256: dict[str, str]
    zig_version: str


class HashedSourcePin(SourcePin):
    source_sha256: str


class SourcePins(TypedDict):
    ghostty: GhosttySourcePin
    kmscon: HashedSourcePin
    uresourced: HashedSourcePin


SOURCE_PINS: SourcePins = json.loads(
    Path(__file__).with_name("source-pins.json").read_text(encoding="utf-8")
)


def github_release_rpm(release: ReleaseRpm) -> PackageResolver:
    def resolve(_: BuildContext) -> tuple[str, ...]:
        arch = fedora_arch()
        if arch is None:
            print(
                f"Skipping {release.name} for unsupported architecture: "
                f"{platform.machine()}",
                file=sys.stderr,
            )
            return ()
        return (release.asset_url(arch),)

    return PackageResolver(resolve)
