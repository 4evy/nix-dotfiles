import hashlib
import json
import os
import platform
import re
import tarfile
import tempfile
import time
from collections.abc import Mapping, Sequence
from pathlib import Path
from typing import Literal

from githubkit import GitHub
from githubkit.exception import GitHubException
from pydantic import BaseModel, ConfigDict, Field, ValidationError
from pydantic_settings import BaseSettings, SettingsConfigDict

from spectrum_build.programs.sources import SOURCE_PINS
from workstation.automation import automation_check_mode
from workstation.automation_models import OperationResult
from workstation.console import console, error_console
from workstation.errors import DotfilesError
from workstation.lib.commands import require_commands, run, which
from workstation.lib.files import (
    ensure_directory,
    extract_tar_archive,
    fresh_directory,
    install_file_if_changed,
    remove_path,
    replace_directory,
    require_directory,
    require_executable,
    require_file,
    write_if_changed,
)
from workstation.lib.http import download
from workstation.lib.paths import find_repo_root

GHOSTTY_PIN = SOURCE_PINS["ghostty"]
GHOSTTY_REVISION = GHOSTTY_PIN["revision"]
GHOSTTY_VERSION = GHOSTTY_PIN["version"]
GHOSTTY_SOURCE_URL = (
    f"https://github.com/ghostty-org/ghostty/archive/{GHOSTTY_REVISION}.tar.gz"
)
GHOSTTY_SOURCE_SHA256 = GHOSTTY_PIN["source_sha256"]
GHOSTTY_ZIG_VERSION = GHOSTTY_PIN["zig_version"]
GHOSTTY_ZIG_SHA256 = GHOSTTY_PIN["zig_sha256"]


def _repo_root() -> Path:
    return find_repo_root(Path(__file__))


def _ghostty_patches() -> tuple[Path, ...]:
    patch_dir = _repo_root() / "patches/ghostty"
    patches = tuple(sorted(patch_dir.glob("*.patch")))
    if not patches:
        raise DotfilesError(f"Ghostty patch series is empty: {patch_dir}")
    return patches


def _ghostty_patch_key(patches: tuple[Path, ...]) -> str:
    digest = hashlib.sha256()
    for patch in patches:
        digest.update(patch.name.encode())
        digest.update(b"\0")
        digest.update(patch.read_bytes())
        digest.update(b"\0")
    return digest.hexdigest()


def _apply_ghostty_patches(source: Path, patches: tuple[Path, ...]) -> None:
    arguments = tuple(os.fspath(patch) for patch in patches)
    result = run(("git", "apply", "--check", *arguments), cwd=source, check=False)
    if result.returncode != 0:
        raise DotfilesError("Ghostty patch series does not apply to the tip source")
    run(("git", "apply", *arguments), cwd=source)


def _merge_install_tree(source: Path, destination: Path) -> None:
    """Merge a staged prefix without copying directory ownership or timestamps."""
    ensure_directory(destination)
    for source_path in source.iterdir():
        destination_path = destination / source_path.name
        if source_path.info.is_symlink():
            remove_path(destination_path)
            destination_path.symlink_to(source_path.readlink())
        elif source_path.info.is_dir():
            if destination_path.is_symlink() or (
                destination_path.exists() and not destination_path.is_dir()
            ):
                remove_path(destination_path)
            _merge_install_tree(source_path, destination_path)
        else:
            if destination_path.is_dir():
                remove_path(destination_path)
            install_file_if_changed(
                source_path,
                destination_path,
                f"{source_path.stat().st_mode & 0o777:04o}",
            )


class InstallerSettings(BaseSettings):
    model_config = SettingsConfigDict(extra="ignore")

    ghostty_tip_check_interval_seconds: int = Field(86400, ge=0)
    ghostty_build_container_image: str = "registry.fedoraproject.org/fedora:latest"
    github_token: str | None = None
    gh_token: str | None = None


def _settings() -> InstallerSettings:
    try:
        return InstallerSettings()
    except ValidationError as error:
        raise DotfilesError(f"invalid installer configuration: {error}") from error


class BuildState(BaseModel):
    """One validated freshness record for a source-built application."""

    model_config = ConfigDict(extra="forbid", frozen=True)

    schema_version: Literal[1] = 1
    revision: str = Field(min_length=1)
    checked_at: int = Field(ge=0)
    inputs: dict[str, str] = Field(default_factory=dict)

    @classmethod
    def read(cls, path: Path) -> BuildState | None:
        try:
            return cls.model_validate_json(path.read_bytes())
        except OSError, ValidationError:
            return None

    @classmethod
    def write(
        cls, path: Path, revision: str, *, inputs: dict[str, str] | None = None
    ) -> BuildState:
        state = cls(
            revision=revision,
            checked_at=int(time.time()),
            inputs=inputs or {},
        )
        write_if_changed(path, state.model_dump_json(indent=2) + "\n")
        return state

    def is_fresh(self, interval: int) -> bool:
        age = int(time.time()) - self.checked_at
        return 0 <= age < interval


def _missing_libraries(executable: Path) -> list[str]:
    if not executable.is_file() or not os.access(executable, os.X_OK):
        return []
    result = run(("ldd", executable), check=False, capture=True, env={"LC_ALL": "C"})
    return [
        line.split()[0]
        for line in result.stdout.splitlines()
        if line.rstrip().endswith("=> not found")
    ]


def _verify_ghostty_runtime(executable: Path) -> None:
    missing = _missing_libraries(executable)
    if not missing:
        return
    details = "\n".join(f"  {name}" for name in missing)
    message = f"Ghostty is missing runtime libraries:\n{details}"
    if "libgtk4-layer-shell.so.0" in missing:
        message += (
            "\nAdd gtk4-layer-shell to the Spectrum image, boot into it, and retry."
        )
    raise DotfilesError(message)


def _ghostty_version_current(executable: Path) -> bool:
    if not executable.is_file() or not os.access(executable, os.X_OK):
        return False
    result = run((executable, "+version"), check=False, capture=True)
    return result.returncode == 0 and GHOSTTY_VERSION in result.stdout


def _zig_architecture() -> str:
    architecture = platform.machine().lower()
    if architecture == "x86_64":
        return "x86_64-linux"
    if architecture in {"aarch64", "arm64"}:
        return "aarch64-linux"
    raise DotfilesError(f"unsupported architecture for Ghostty build: {architecture}")


def _rewrite_ghostty_files(prefix: Path, executable: Path) -> None:
    desktop = prefix / "share/applications/com.mitchellh.ghostty.desktop"
    if desktop.is_file():
        content = desktop.read_text()
        content = re.sub(
            r"^TryExec=.*$", f"TryExec={executable}", content, flags=re.MULTILINE
        )
        content = re.sub(
            r"^Exec=.*ghostty --gtk-single-instance=true$",
            f"Exec={executable} --gtk-single-instance=true",
            content,
            flags=re.MULTILINE,
        )
        content = re.sub(
            r"^DBusActivatable=.*$",
            "DBusActivatable=false",
            content,
            flags=re.MULTILINE,
        )
        write_if_changed(desktop, content)
    for service in (
        prefix / "share/dbus-1/services/com.mitchellh.ghostty.service",
        prefix / "share/systemd/user/app-com.mitchellh.ghostty.service",
    ):
        if service.is_file():
            content = service.read_text().replace(
                "Exec=/work/stage/bin/ghostty", f"Exec={executable}"
            )
            content = content.replace(
                "ExecStart=/work/stage/bin/ghostty", f"ExecStart={executable}"
            )
            write_if_changed(service, content)


def _extract_application_directory(
    archive_path: Path, destination: Path, *, label: str
) -> Path:
    with tarfile.open(archive_path) as archive:
        extract_tar_archive(archive, destination)
    extracted = next(
        (path for path in destination.iterdir() if path.info.is_dir()), None
    )
    if extracted is None:
        raise DotfilesError(f"{label} archive did not contain an application directory")
    return extracted


def _run_logged_build(
    argv: Sequence[str | os.PathLike[str]],
    build_log: Path,
    *,
    label: str,
    cwd: str | Path | None = None,
    env: Mapping[str, str] | None = None,
) -> None:
    result = run(argv, cwd=cwd, env=env, check=False, capture=True)
    write_if_changed(build_log, result.stdout + result.stderr)
    if result.returncode != 0:
        tail = "\n".join(build_log.read_text(encoding="utf-8").splitlines()[-160:])
        raise DotfilesError(
            f"{label} build failed; tail of {build_log} follows:\n{tail}"
        )


def _build_ghostty(
    cache_dir: Path,
    install_prefix: Path,
    executable: Path,
    patches: tuple[Path, ...],
    container_image: str,
) -> None:
    build_log = cache_dir / "ghostty-tip-build.log"
    with tempfile.TemporaryDirectory(prefix="build-", dir=cache_dir) as temporary:
        work = Path(temporary)
        source_dir = ensure_directory(work / "source")
        stage_dir = ensure_directory(work / "stage")
        source_archive = work / "ghostty-source.tar.gz"
        download(
            GHOSTTY_SOURCE_URL,
            source_archive,
            expected_sha256=GHOSTTY_SOURCE_SHA256,
        )
        extracted = _extract_application_directory(
            source_archive, source_dir, label="Ghostty source"
        )
        ghostty_source = work / "ghostty"
        extracted.replace(ghostty_source)
        _apply_ghostty_patches(ghostty_source, patches)
        container_script = (
            _repo_root()
            / "ansible/files/scripts/apps/install-ghostty-tip-linux.container.py"
        )
        zig_architecture = _zig_architecture()
        _run_logged_build(
            (
                "podman",
                "run",
                "--rm",
                "--security-opt",
                "label=disable",
                "--volume",
                f"{work}:/work",
                "--volume",
                f"{container_script}:/tmp/ghostty-build.py:ro",
                "--workdir",
                "/work/ghostty",
                "--env",
                f"ZIG_ARCH={zig_architecture}",
                "--env",
                f"ZIG_VERSION={GHOSTTY_ZIG_VERSION}",
                "--env",
                f"ZIG_SHA256={GHOSTTY_ZIG_SHA256[zig_architecture]}",
                "--env",
                f"GHOSTTY_VERSION={GHOSTTY_VERSION}",
                container_image,
                "sh",
                "-ceu",
                (
                    "dnf -y install --setopt=install_weak_deps=False python3 && "
                    "exec python3 /tmp/ghostty-build.py"
                ),
            ),
            build_log,
            label="Ghostty",
        )
        built_binary = stage_dir / "bin/ghostty"
        pending = work / ".ghostty-bin"
        if built_binary.is_file():
            built_binary.replace(pending)
        _merge_install_tree(stage_dir, install_prefix)
        if pending.is_file():
            ensure_directory(executable.parent)
            install_file_if_changed(
                pending,
                executable,
                f"{pending.stat().st_mode & 0o777:04o}",
            )


def install_ghostty_tip_linux(
    cache_dir: Path,
    install_prefix: Path,
) -> OperationResult:
    """Build the Ghostty tip release in a disposable Fedora container."""
    executable = install_prefix / "bin/ghostty"
    state_path = install_prefix / ".ghostty-tip-state.json"
    patches = _ghostty_patches()
    patch_key = _ghostty_patch_key(patches)
    settings = _settings()
    interval = settings.ghostty_tip_check_interval_seconds
    state = BuildState.read(state_path)
    current = (
        state is not None
        and state.revision == GHOSTTY_REVISION
        and state.inputs == {"patches": patch_key}
        and executable.is_file()
        and not _missing_libraries(executable)
        and _ghostty_version_current(executable)
    )
    fresh = current and state.is_fresh(interval)
    if automation_check_mode():
        return OperationResult(
            changed=not fresh,
            msg=(
                "Ghostty tip is current"
                if fresh
                else "Would check and install the current Ghostty tip"
            ),
        )
    require_commands("git", "podman")
    cache_dir = ensure_directory(cache_dir)
    install_prefix = ensure_directory(install_prefix)
    if fresh:
        console.print(
            f"Ghostty tip was checked less than {interval} seconds ago; skipping."
        )
        return OperationResult(msg="Ghostty tip was checked recently")
    if executable.exists():
        _verify_ghostty_runtime(executable)
    if current:
        BuildState.write(
            state_path,
            GHOSTTY_REVISION,
            inputs={"patches": patch_key},
        )
        console.print("Ghostty tip source is already current.")
        return OperationResult(
            msg="Ghostty tip source is already current",
            data={"source_key": GHOSTTY_REVISION},
        )

    _build_ghostty(
        cache_dir,
        install_prefix,
        executable,
        patches,
        settings.ghostty_build_container_image,
    )

    _rewrite_ghostty_files(install_prefix, executable)
    desktop_dir = install_prefix / "share/applications"
    if which("update-desktop-database") is not None and desktop_dir.is_dir():
        run(("update-desktop-database", desktop_dir), check=False, capture=True)
    _verify_ghostty_runtime(executable)
    BuildState.write(
        state_path,
        GHOSTTY_REVISION,
        inputs={"patches": patch_key},
    )
    console.print(f"Installed native Ghostty tip release build into {install_prefix}.")
    return OperationResult(
        changed=True,
        msg=f"Installed native Ghostty tip release build into {install_prefix}",
        data={"source_key": GHOSTTY_REVISION},
    )


def _ghostty_macos_toolchain() -> str:
    result = run(
        ("/usr/bin/xcodebuild", "-showComponent", "MetalToolchain", "-json"),
        capture=True,
    )
    try:
        component = json.loads(result.stdout)
        status = component["status"]
        identifier = component["toolchainIdentifier"]
    except (json.JSONDecodeError, KeyError, TypeError) as error:
        raise DotfilesError(
            "Xcode returned invalid Metal toolchain metadata"
        ) from error
    if status != "installed":
        raise DotfilesError(
            "Xcode Metal toolchain is not installed; run the prerequisites role"
        )
    if not isinstance(identifier, str) or not identifier:
        raise DotfilesError("Xcode returned an invalid Metal toolchain identifier")
    return identifier


def _ghostty_macos_patch_current(executable: Path) -> bool:
    if not executable.is_file() or not os.access(executable, os.X_OK):
        return False
    result = run(
        (executable, "+show-config", "--default", "--no-pager"),
        check=False,
        capture=True,
    )
    return result.returncode == 0 and any(
        line.partition("=")[0].strip() == "scrollback-editor"
        for line in result.stdout.splitlines()
    )


def _build_ghostty_macos(
    cache_dir: Path,
    app_root: Path,
    zig_executable: Path,
    patches: tuple[Path, ...],
) -> None:
    build_log = cache_dir / "ghostty-tip-macos-build.log"
    source_archive = cache_dir / f"ghostty-{GHOSTTY_REVISION}.tar.gz"
    _verified_download(
        source_archive,
        GHOSTTY_SOURCE_URL,
        f"sha256:{GHOSTTY_SOURCE_SHA256}",
    )
    with tempfile.TemporaryDirectory(prefix="build-", dir=cache_dir) as temporary:
        work = Path(temporary)
        source_dir = ensure_directory(work / "source")
        extracted = _extract_application_directory(
            source_archive, source_dir, label="Ghostty source"
        )
        ghostty_source = work / "ghostty"
        extracted.replace(ghostty_source)
        _apply_ghostty_patches(ghostty_source, patches)
        _run_logged_build(
            (
                zig_executable,
                "build",
                "-Doptimize=ReleaseFast",
                "-Dxcframework-target=native",
                f"-Dversion-string={GHOSTTY_VERSION}",
            ),
            build_log,
            label="Ghostty macOS",
            cwd=ghostty_source,
            env={
                "TOOLCHAINS": _ghostty_macos_toolchain(),
                "ZIG_GLOBAL_CACHE_DIR": os.fspath(cache_dir / "zig"),
            },
        )
        replace_directory(
            ghostty_source / "zig-out/Ghostty.app", app_root / "Ghostty.app"
        )


def install_ghostty_tip_macos(
    cache_dir: Path,
    app_root: Path,
    zig_executable: Path,
) -> OperationResult:
    """Build and install the patched native Ghostty macOS application."""
    app_dir = app_root / "Ghostty.app"
    executable = app_dir / "Contents/MacOS/ghostty"
    state_path = app_root / ".ghostty-tip-state.json"
    patches = _ghostty_patches()
    patch_key = _ghostty_patch_key(patches)
    state = BuildState.read(state_path)
    valid_install = _ghostty_version_current(
        executable
    ) and _ghostty_macos_patch_current(executable)
    current = (
        state is not None
        and state.revision == GHOSTTY_REVISION
        and state.inputs == {"patches": patch_key, "target": "native-macos"}
        and valid_install
    )
    if automation_check_mode():
        return OperationResult(
            changed=not current,
            msg=(
                "Ghostty macOS tip is current"
                if current
                else "Would build and install the patched Ghostty macOS tip"
            ),
        )
    if current:
        return OperationResult(msg="Ghostty macOS tip is current")

    require_commands("/usr/bin/codesign")
    if valid_install:
        run(("/usr/bin/codesign", "--verify", "--deep", "--strict", app_dir))
        BuildState.write(
            state_path,
            GHOSTTY_REVISION,
            inputs={"patches": patch_key, "target": "native-macos"},
        )
        return OperationResult(
            changed=True,
            msg="Adopted the existing patched Ghostty macOS build",
            data={"source_key": GHOSTTY_REVISION},
        )

    require_commands("git", "/usr/bin/xcodebuild")
    zig_executable = require_executable(zig_executable)
    cache_dir = ensure_directory(cache_dir)
    app_root = ensure_directory(app_root)
    ensure_directory(cache_dir / "zig")
    _build_ghostty_macos(cache_dir, app_root, zig_executable, patches)
    run(("/usr/bin/codesign", "--verify", "--deep", "--strict", app_dir))
    if not _ghostty_version_current(executable) or not _ghostty_macos_patch_current(
        executable
    ):
        raise DotfilesError("installed Ghostty macOS build failed patch verification")
    BuildState.write(
        state_path,
        GHOSTTY_REVISION,
        inputs={"patches": patch_key, "target": "native-macos"},
    )
    console.print(f"Installed patched Ghostty macOS build into {app_root}.")
    return OperationResult(
        changed=True,
        msg=f"Installed patched Ghostty macOS build into {app_root}",
        data={"source_key": GHOSTTY_REVISION},
    )


def _helium_architecture() -> str:
    architecture = platform.machine().lower()
    if architecture in {"arm64", "aarch64"}:
        return "arm64"
    if architecture in {"x86_64", "amd64"}:
        return "x86_64"
    raise DotfilesError(f"unsupported Helium architecture: {architecture}")


def _verified_download(path: Path, url: str, digest: str | None) -> None:
    expected = (
        digest.removeprefix("sha256:").lower()
        if digest and digest.startswith("sha256:")
        else None
    )
    if expected and not re.fullmatch(r"[0-9a-f]{64}", expected):
        raise DotfilesError(f"invalid Helium sha256 digest: {digest}")
    if path.is_file() and (
        expected is None or hashlib.sha256(path.read_bytes()).hexdigest() == expected
    ):
        error_console.print(f"helium-browser: using cached {path.name}")
        return
    error_console.print(f"helium-browser: downloading {path.name}")
    download(url, path, expected_sha256=expected)


def _helium_apply_input(secrets: Path) -> str:
    apply_input: dict[str, object] = {}
    decrypted = run(
        (
            "sops",
            "--decrypt",
            "--output-type",
            "json",
            "--extract",
            '["chromium-cookie-allowed-for-urls"]',
            secrets,
        ),
        check=False,
        capture=True,
    )
    if decrypted.returncode == 0:
        try:
            cookie_allowlist = json.loads(decrypted.stdout)
        except json.JSONDecodeError as error:
            raise DotfilesError(
                "Chromium cookie allowlist is not valid JSON"
            ) from error
        if not isinstance(cookie_allowlist, list) or not all(
            isinstance(pattern, str) for pattern in cookie_allowlist
        ):
            raise DotfilesError("Chromium cookie allowlist must be an array of strings")
        apply_input["cookie_allowlist"] = cookie_allowlist
    else:
        error_console.print(
            "helium-browser: private cookie settings could not be decrypted; "
            "continuing with public settings"
        )

    if which("gh") is not None:
        token = run(("gh", "auth", "token"), check=False, capture=True)
        value = token.stdout.strip()
        if token.returncode == 0 and value:
            apply_input["extension_values"] = {
                "refined-github-personal-token": value,
            }
    return json.dumps(apply_input)


def _run_helium_configurer(
    *,
    platform_name: str,
    root: Path,
    app_dir: Path,
    bin_dir: Path,
    installer_bin: Path,
    secrets: Path,
    flags: str,
) -> None:
    require_commands("go", "sops")
    ensure_directory(installer_bin)
    run(
        ("go", "install", "./cmd/helium-browser"),
        cwd=_repo_root(),
        env={
            "GOBIN": os.fspath(installer_bin),
            "CGO_ENABLED": "0"
            if platform_name == "linux"
            else os.environ.get("CGO_ENABLED", "1"),
        },
    )
    run(
        (
            installer_bin / "helium-browser",
            "configure",
            "--input",
            "-",
            "--",
            platform_name,
            root,
            app_dir,
            bin_dir,
            flags,
        ),
        cwd=_repo_root(),
        input_text=_helium_apply_input(secrets),
    )


def install_helium_linux(
    cache_root: Path,
    app_root: Path,
    bin_dir: Path,
    installer_bin: Path,
    secrets_file: Path,
    flags: str = "",
) -> OperationResult:
    """Install the latest verified Helium Linux release and configure it."""
    if automation_check_mode():
        return OperationResult(changed=True, msg="Would reconcile Helium on Linux")
    cache = ensure_directory(cache_root)
    app_root = ensure_directory(app_root)
    bin_dir = ensure_directory(bin_dir)
    secrets = require_file(secrets_file)
    settings = _settings()
    try:
        release = (
            GitHub(
                settings.github_token or settings.gh_token,
                user_agent="dotfiles-helium-installer",
                timeout=60,
            )
            .rest.repos.get_latest_release("imputnet", "helium-linux")
            .parsed_data
        )
    except GitHubException as error:
        raise DotfilesError(
            f"failed to read the latest Helium release: {error}"
        ) from error
    version = release.tag_name
    name = f"helium-{version}-{_helium_architecture()}_linux.tar.xz"
    asset = next(
        (candidate for candidate in release.assets if candidate.name == name), None
    )
    if asset is None:
        raise DotfilesError(f"latest Helium release does not include {name}")
    archive = cache / name
    app_dir = app_root / "app"
    version_file = app_dir / ".helium-version"
    if not version_file.is_file() or version_file.read_text().strip() != version:
        _verified_download(archive, str(asset.browser_download_url), asset.digest)
        extract = fresh_directory(cache / "extract")
        payload = _extract_application_directory(archive, extract, label="Helium")
        replace_directory(payload, app_dir)
        wrapper = app_dir / "helium-wrapper"
        if wrapper.is_file():
            write_if_changed(
                wrapper,
                re.sub(
                    r"^CHROME_VERSION_EXTRA=.*$",
                    "CHROME_VERSION_EXTRA=ansible",
                    wrapper.read_text(),
                    flags=re.MULTILINE,
                ),
                "0755",
            )
        write_if_changed(version_file, version + "\n")
        remove_path(extract)
    _run_helium_configurer(
        platform_name="linux",
        root=cache,
        app_dir=app_dir,
        bin_dir=bin_dir,
        installer_bin=installer_bin,
        secrets=secrets,
        flags=flags,
    )
    return OperationResult(changed=True, msg=f"Configured Helium {version} on Linux")


def install_helium_macos(
    root: Path, bin_dir: Path, installer_bin: Path, secrets_file: Path, flags: str = ""
) -> OperationResult:
    """Configure the Homebrew Helium macOS application."""
    if automation_check_mode():
        return OperationResult(changed=True, msg="Would reconcile Helium on macOS")
    root = ensure_directory(root)
    bin_dir = ensure_directory(bin_dir)
    app_dir = require_directory("/Applications/Helium.app")
    _run_helium_configurer(
        platform_name="macos",
        root=root,
        app_dir=app_dir,
        bin_dir=bin_dir,
        installer_bin=installer_bin,
        secrets=require_file(secrets_file),
        flags=flags,
    )
    return OperationResult(changed=True, msg="Configured Helium on macOS")
