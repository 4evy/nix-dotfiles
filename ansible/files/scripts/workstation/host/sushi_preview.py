import shutil
from pathlib import Path

from pydantic import BaseModel, ConfigDict, Field, JsonValue, ValidationError
from pydantic_settings import BaseSettings, SettingsConfigDict

from workstation.automation import automation_check_mode
from workstation.automation_models import OperationResult
from workstation.console import console
from workstation.errors import DotfilesError
from workstation.lib.commands import require_commands, run, which
from workstation.lib.files import ensure_directory, write_if_changed
from workstation.lib.host import user_cache_home, user_state_home


class SushiSettings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SUSHI_PREVIEW_", extra="ignore")

    app_id: str = "org.gnome.NautilusPreviewer"
    profile: str = "default"
    rev: str = "127eb8e45115d257c6bb0254b4f2e5f37bc7233d"
    repo: str = "https://github.com/GNOME/sushi.git"


class SushiModule(BaseModel):
    model_config = ConfigDict(extra="allow")

    name: str | None = None
    config_opts: list[JsonValue] | None = Field(None, alias="config-opts")


class SushiManifest(BaseModel):
    model_config = ConfigDict(extra="allow")

    app_id: str | None = Field(None, alias="app-id")
    modules: list[SushiModule]


def update_manifest(path: Path, *, app_id: str, profile: str) -> None:
    try:
        manifest = SushiManifest.model_validate_json(path.read_bytes())
    except (OSError, ValidationError) as error:
        raise DotfilesError(f"invalid Sushi Flatpak manifest: {path}") from error
    manifest.app_id = app_id
    for module in manifest.modules:
        if module.name != "sushi":
            continue
        options = module.config_opts or []
        module.config_opts = options
        setting = f"-Dprofile={profile}"
        replaced = False
        for index, option in enumerate(options):
            if isinstance(option, str) and option.startswith("-Dprofile="):
                options[index] = setting
                replaced = True
        if not replaced:
            options.append(setting)
    write_if_changed(
        path,
        manifest.model_dump_json(by_alias=True, exclude_none=True, indent=2) + "\n",
    )


def install(settings: SushiSettings | None = None) -> OperationResult:
    config = settings or SushiSettings()

    cache_root = user_cache_home() / "dotfiles/sushi-preview-flatpak"
    source_dir = cache_root / "source"
    build_dir = cache_root / "build"
    builder_state_dir = cache_root / "flatpak-builder-state"
    manifest_file = source_dir / "flatpak/org.gnome.NautilusPreviewer.json"
    stamp_dir = user_state_home() / "dotfiles"
    stamp_file = stamp_dir / "sushi-preview-flatpak.stamp"
    stamp = f"rev={config.rev} app_id={config.app_id} profile={config.profile}"

    if automation_check_mode() and (which("flatpak") is None or which("git") is None):
        return OperationResult(
            changed=True,
            msg=f"Would install prerequisites and {config.app_id} from {config.rev}",
            data={"app_id": config.app_id, "revision": config.rev},
        )
    require_commands("flatpak", "git")

    installed = (
        run(
            ("flatpak", "info", "--user", config.app_id), check=False, capture=True
        ).returncode
        == 0
    )
    if stamp_file.is_file() and stamp_file.read_text().strip() == stamp and installed:
        console.print(
            f"{config.app_id} is already installed from {config.rev} "
            f"with profile {config.profile}"
        )
        return OperationResult(
            msg=f"{config.app_id} is already installed",
            data={"app_id": config.app_id, "revision": config.rev},
        )
    if automation_check_mode():
        return OperationResult(
            changed=True,
            msg=f"Would install {config.app_id} from {config.rev}",
            data={"app_id": config.app_id, "revision": config.rev},
        )

    run(
        ("flatpak", "override", "--user", "--env=GDK_GL=gles", config.app_id),
        check=False,
    )

    ensure_directory(cache_root, "0755")
    ensure_directory(stamp_dir, "0755")
    builder = run(
        ("flatpak", "info", "--user", "org.flatpak.Builder"),
        check=False,
        capture=True,
    )
    if builder.returncode != 0:
        raise DotfilesError(
            "Sushi Flatpak build prerequisites are missing; run the host_layer "
            "Ansible role with the sushi-preview tag"
        )

    if (source_dir / ".git").is_dir():
        run((
            "git",
            "-C",
            source_dir,
            "fetch",
            "--tags",
            "--prune",
            "--filter=blob:none",
            "origin",
        ))
    else:
        if source_dir.exists():
            shutil.rmtree(source_dir)
        run((
            "git",
            "clone",
            "--filter=blob:none",
            "--no-checkout",
            "--",
            config.repo,
            source_dir,
        ))
    run(("git", "-C", source_dir, "checkout", "--force", "--detach", config.rev))
    run(("git", "-C", source_dir, "clean", "-fdx"))
    update_manifest(manifest_file, app_id=config.app_id, profile=config.profile)

    run((
        "flatpak",
        "run",
        f"--filesystem={cache_root}",
        "org.flatpak.Builder",
        "--user",
        "--install",
        "--force-clean",
        f"--state-dir={builder_state_dir}",
        build_dir,
        manifest_file,
    ))
    run(("flatpak", "override", "--user", "--env=GDK_GL=gles", config.app_id))
    run(("flatpak", "kill", config.app_id), check=False, capture=True)
    if which("nautilus") is not None:
        run(("nautilus", "-q"), check=False, capture=True)
    write_if_changed(stamp_file, stamp + "\n")
    console.print(
        f"installed {config.app_id} from {config.rev} with profile {config.profile}"
    )
    return OperationResult(
        changed=True,
        msg=f"Installed {config.app_id} from {config.rev}",
        data={"app_id": config.app_id, "revision": config.rev},
    )
