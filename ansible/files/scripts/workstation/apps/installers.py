from __future__ import annotations

import hashlib
import os
import platform
import re
import shutil
import tarfile
import tempfile
import time
from pathlib import Path
from typing import Annotated, Any

import typer

from workstation.automation import automation_check_mode
from workstation.automation_models import OperationResult
from workstation.console import console, error_console
from workstation.errors import DotfilesError
from workstation.lib.commands import output, require_commands, run, which
from workstation.lib.files import (
    ensure_directory,
    extract_tar_archive,
    fresh_directory,
    install_file_if_changed,
    replace_directory,
    require_directory,
    require_file,
    write_if_changed,
)
from workstation.lib.http import download, get
from workstation.lib.paths import find_repo_root


def _repo_root() -> Path:
    return find_repo_root(Path(__file__))


def _nonnegative_integer(value: str, name: str) -> int:
    if not value.isdigit():
        raise DotfilesError(f"{name} must be a nonnegative integer: {value}")
    return int(value)


def _fresh_check(
    *,
    executable: Path,
    checked_at: Path,
    state_file: Path,
    state_version: str,
    interval: int,
    extra: tuple[bool, ...] = (),
) -> bool:
    if not executable.is_file() or not os.access(executable, os.X_OK):
        return False
    if not checked_at.is_file() or not state_file.is_file():
        return False
    if state_file.read_text(encoding="utf-8").strip() != state_version or not all(
        extra
    ):
        return False
    checked = checked_at.read_text(encoding="utf-8").strip()
    return checked.isdigit() and int(time.time()) - int(checked) < interval


def _write_check_state(
    *, state_key: str, key_file: Path, checked_at: Path, state_file: Path, version: str
) -> None:
    write_if_changed(key_file, state_key + "\n")
    write_if_changed(checked_at, f"{int(time.time())}\n")
    write_if_changed(state_file, version + "\n")


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


def install_ghostty_tip_linux(
    cache_dir: Annotated[Path, typer.Argument()],
    install_prefix: Annotated[Path, typer.Argument()],
) -> OperationResult:
    """Build the Ghostty tip release in a disposable Fedora container."""
    cache = cache_dir
    prefix = install_prefix
    executable = prefix / "bin/ghostty"
    checked_at = prefix / ".ghostty-tip-checked-at"
    source_key_file = prefix / ".ghostty-tip-source-key"
    state_file = prefix / ".ghostty-tip-state-version"
    interval = _nonnegative_integer(
        os.environ.get("GHOSTTY_TIP_CHECK_INTERVAL_SECONDS", "86400"),
        "GHOSTTY_TIP_CHECK_INTERVAL_SECONDS",
    )
    fresh = _fresh_check(
        executable=executable,
        checked_at=checked_at,
        state_file=state_file,
        state_version="1",
        interval=interval,
        extra=(not _missing_libraries(executable), source_key_file.is_file()),
    )
    if automation_check_mode():
        return OperationResult(
            changed=not fresh,
            msg=(
                "Ghostty tip is current"
                if fresh
                else "Would check and install the current Ghostty tip"
            ),
        )
    require_commands("gh", "podman")
    cache = ensure_directory(cache)
    prefix = ensure_directory(prefix)
    if fresh:
        console.print(
            f"Ghostty tip was checked less than {interval} seconds ago; skipping."
        )
        return OperationResult(msg="Ghostty tip was checked recently")
    if executable.exists():
        _verify_ghostty_runtime(executable)
    if run(("gh", "auth", "status"), check=False, capture=True).returncode != 0:
        raise DotfilesError("gh must be authenticated; run `gh auth login` and retry")

    source_key = output((
        "gh",
        "release",
        "view",
        "tip",
        "--repo",
        "ghostty-org/ghostty",
        "--json",
        "assets",
        "--jq",
        '.assets[] | select(.name == "ghostty-source.tar.gz") | [.name, .size, .updatedAt] | @tsv',
    ))
    if not source_key:
        raise DotfilesError(
            "Ghostty tip release does not include ghostty-source.tar.gz"
        )
    if (
        executable.is_file()
        and source_key_file.is_file()
        and source_key_file.read_text().strip() == source_key
    ):
        _verify_ghostty_runtime(executable)
        _write_check_state(
            state_key=source_key,
            key_file=source_key_file,
            checked_at=checked_at,
            state_file=state_file,
            version="1",
        )
        console.print("Ghostty tip source is already current.")
        return OperationResult(
            msg="Ghostty tip source is already current",
            data={"source_key": source_key},
        )

    build_log = cache / "ghostty-tip-build.log"
    with tempfile.TemporaryDirectory(prefix="build-", dir=cache) as temporary:
        work = Path(temporary)
        download_dir = ensure_directory(work / "download")
        source_dir = ensure_directory(work / "source")
        stage_dir = ensure_directory(work / "stage")
        run((
            "gh",
            "release",
            "download",
            "tip",
            "--repo",
            "ghostty-org/ghostty",
            "--pattern",
            "ghostty-source.tar.gz",
            "--clobber",
            "--dir",
            download_dir,
        ))
        with tarfile.open(download_dir / "ghostty-source.tar.gz") as archive:
            extract_tar_archive(archive, source_dir)
        extracted = next((path for path in source_dir.iterdir() if path.is_dir()), None)
        if extracted is None:
            raise DotfilesError(
                "Ghostty source tarball did not contain a source directory"
            )
        ghostty_source = work / "ghostty"
        extracted.replace(ghostty_source)
        container_script = (
            _repo_root()
            / "ansible/files/scripts/apps/install-ghostty-tip-linux.container.py"
        )
        result = run(
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
                f"ZIG_ARCH={_zig_architecture()}",
                "--env",
                "ZIG_VERSION=0.15.2",
                os.environ.get(
                    "GHOSTTY_BUILD_CONTAINER_IMAGE",
                    "registry.fedoraproject.org/fedora:latest",
                ),
                "python3",
                "/tmp/ghostty-build.py",
            ),
            check=False,
            capture=True,
        )
        write_if_changed(build_log, result.stdout + result.stderr)
        if result.returncode != 0:
            tail = "\n".join(build_log.read_text().splitlines()[-160:])
            raise DotfilesError(
                f"Ghostty build failed; tail of {build_log} follows:\n{tail}"
            )
        built_binary = stage_dir / "bin/ghostty"
        pending = work / ".ghostty-bin"
        if built_binary.is_file():
            built_binary.replace(pending)
        shutil.copytree(stage_dir, prefix, dirs_exist_ok=True, symlinks=True)
        if pending.is_file():
            ensure_directory(executable.parent)
            install_file_if_changed(pending, executable, pending.stat().st_mode & 0o777)

    _rewrite_ghostty_files(prefix, executable)
    desktop_dir = prefix / "share/applications"
    if which("update-desktop-database") is not None and desktop_dir.is_dir():
        run(("update-desktop-database", desktop_dir), check=False, capture=True)
    _verify_ghostty_runtime(executable)
    _write_check_state(
        state_key=source_key,
        key_file=source_key_file,
        checked_at=checked_at,
        state_file=state_file,
        version="1",
    )
    console.print(f"Installed native Ghostty tip release build into {prefix}.")
    return OperationResult(
        changed=True,
        msg=f"Installed native Ghostty tip release build into {prefix}",
        data={"source_key": source_key},
    )


def _is_full_sha(value: str) -> bool:
    return re.fullmatch(r"[0-9a-f]{40}", value) is not None


def install_helix_tip_linux(
    cache_dir: Annotated[Path, typer.Argument()],
    install_prefix: Annotated[Path, typer.Argument()],
) -> OperationResult:
    """Build and install the pinned Helix tip revision."""
    cache = cache_dir
    prefix = install_prefix
    source_ref = os.environ.get(
        "HELIX_TIP_REVISION", "14d6bc0febed9c692048271a8ae2362ac969c6e0"
    )
    interval = _nonnegative_integer(
        os.environ.get("HELIX_TIP_CHECK_INTERVAL_SECONDS", "86400"),
        "HELIX_TIP_CHECK_INTERVAL_SECONDS",
    )
    repo = cache / "source"
    build_log = cache / "helix-tip-build.log"
    stamp = prefix / ".helix-tip-revision"
    checked = prefix / ".helix-tip-checked-at"
    state = prefix / ".helix-tip-state-version"
    executable = prefix / "bin/hx"
    fresh_extra = (stamp.is_file(),)
    if _is_full_sha(source_ref):
        fresh_extra += (stamp.is_file() and stamp.read_text().strip() == source_ref,)
    fresh = _fresh_check(
        executable=executable,
        checked_at=checked,
        state_file=state,
        state_version="2",
        interval=interval,
        extra=fresh_extra,
    )
    if automation_check_mode():
        return OperationResult(
            changed=not fresh,
            msg=(
                "Helix tip is current"
                if fresh
                else f"Would install Helix tip {source_ref}"
            ),
            data={"source_ref": source_ref},
        )
    require_commands("cargo", "git")
    cache = ensure_directory(cache)
    prefix = ensure_directory(prefix)
    if fresh:
        console.print(
            f"Helix tip was checked less than {interval} seconds ago; skipping."
        )
        return OperationResult(
            msg="Helix tip was checked recently", data={"source_ref": source_ref}
        )
    if not (repo / ".git").is_dir():
        if repo.exists():
            shutil.rmtree(repo)
        run(("git", "init", repo))
        run((
            "git",
            "-C",
            repo,
            "remote",
            "add",
            "origin",
            "https://github.com/helix-editor/helix.git",
        ))
    if _is_full_sha(source_ref):
        if (
            run(
                ("git", "-C", repo, "cat-file", "-e", f"{source_ref}^{{commit}}"),
                check=False,
                capture=True,
            ).returncode
            != 0
        ):
            run(("git", "-C", repo, "fetch", "--depth=1", "origin", source_ref))
        checkout = source_ref
    else:
        run((
            "git",
            "-C",
            repo,
            "fetch",
            "--depth=1",
            "--prune",
            "origin",
            f"+refs/heads/{source_ref}:refs/remotes/origin/{source_ref}",
        ))
        checkout = f"origin/{source_ref}"
    run(("git", "-C", repo, "checkout", "--force", checkout))
    revision = output(("git", "-C", repo, "rev-parse", "HEAD"))
    if (
        executable.is_file()
        and stamp.is_file()
        and stamp.read_text().strip() == revision
    ):
        _write_check_state(
            state_key=revision,
            key_file=stamp,
            checked_at=checked,
            state_file=state,
            version="2",
        )
        console.print(f"Helix tip already current at {revision}.")
        return OperationResult(
            msg=f"Helix tip is already current at {revision}",
            data={"revision": revision},
        )
    runtime = prefix / "libexec/runtime"
    if runtime.exists():
        shutil.rmtree(runtime)
    result = run(
        (
            "cargo",
            "install",
            "--path",
            "helix-term",
            "--locked",
            "--profile",
            "opt",
            "--force",
            "--root",
            prefix,
        ),
        cwd=repo,
        env={"HELIX_DEFAULT_RUNTIME": os.fspath(runtime)},
        check=False,
        capture=True,
    )
    write_if_changed(build_log, result.stdout + result.stderr)
    if result.returncode != 0:
        tail = "\n".join(build_log.read_text().splitlines()[-160:])
        raise DotfilesError(f"Helix build failed; tail of {build_log} follows:\n{tail}")
    grammar_sources = repo / "runtime/grammars/sources"
    if grammar_sources.exists():
        shutil.rmtree(grammar_sources)
    replace_directory(repo / "runtime", runtime)
    _write_check_state(
        state_key=revision,
        key_file=stamp,
        checked_at=checked,
        state_file=state,
        version="2",
    )
    console.print(f"Installed Helix tip {revision} into {prefix}.")
    return OperationResult(
        changed=True,
        msg=f"Installed Helix tip {revision} into {prefix}",
        data={"revision": revision},
    )


def _helium_architecture() -> str:
    architecture = platform.machine().lower()
    if architecture in {"arm64", "aarch64"}:
        return "arm64"
    if architecture in {"x86_64", "amd64"}:
        return "x86_64"
    raise DotfilesError(f"unsupported Helium architecture: {architecture}")


def _helium_asset(release: dict[str, Any], name: str) -> dict[str, Any]:
    for asset in release.get("assets", []):
        if asset.get("name") == name:
            return asset
    raise DotfilesError(f"latest Helium release does not include {name}")


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
    download(url, path)
    if expected and hashlib.sha256(path.read_bytes()).hexdigest() != expected:
        path.unlink(missing_ok=True)
        raise DotfilesError(f"Helium sha256 mismatch for {path.name}")


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
            "--secrets",
            secrets,
            "--",
            platform_name,
            root,
            app_dir,
            bin_dir,
            flags,
        ),
        cwd=_repo_root(),
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
    release = get(
        "https://api.github.com/repos/imputnet/helium-linux/releases/latest"
    ).json()
    version = release["tag_name"]
    name = f"helium-{version}-{_helium_architecture()}_linux.tar.xz"
    asset = _helium_asset(release, name)
    archive = cache / name
    app_dir = app_root / "app"
    version_file = app_dir / ".helium-version"
    if not version_file.is_file() or version_file.read_text().strip() != version:
        _verified_download(archive, asset["browser_download_url"], asset.get("digest"))
        extract = fresh_directory(cache / "extract")
        with tarfile.open(archive) as source:
            extract_tar_archive(source, extract)
        payload = next((path for path in extract.iterdir() if path.is_dir()), None)
        if payload is None:
            raise DotfilesError(
                "Helium archive did not contain an application directory"
            )
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
        shutil.rmtree(extract)
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
