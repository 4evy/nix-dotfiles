# dotfiles 🌸

Personal Spectrum/Bluefin, macOS, Homebrew, Ansible, and chezmoi setup.

<p>
  <a href="https://github.com/4evy/dotfiles/pkgs/container/spectrum"><img alt="GHCR Spectrum image" src="https://img.shields.io/badge/GHCR-spectrum-f4b8e4?style=flat-square&logo=github&logoColor=f4b8e4&labelColor=232634"></a>
</p>

<p>
  <a href="https://projectbluefin.io"><img alt="Bluefin" src="https://img.shields.io/badge/Bluefin-f4b8e4?style=flat-square&logo=fedora&logoColor=f4b8e4&labelColor=232634"></a>
  <a href="https://nixos.org"><img alt="NixOS" src="https://img.shields.io/badge/NixOS-f4b8e4?style=flat-square&logo=nixos&logoColor=f4b8e4&labelColor=232634"></a>
  <a href="https://www.apple.com/macos"><img alt="macOS" src="https://img.shields.io/badge/macOS-f4b8e4?style=flat-square&logo=apple&logoColor=f4b8e4&labelColor=232634"></a>
</p>

<p>
  <a href="https://brew.sh"><img alt="Homebrew" src="https://img.shields.io/badge/Homebrew-f4b8e4?style=flat-square&logo=homebrew&logoColor=f4b8e4&labelColor=232634"></a>
  <a href="https://www.chezmoi.io"><img alt="chezmoi" src="https://img.shields.io/badge/chezmoi-f4b8e4?style=flat-square&logo=homeassistant&logoColor=f4b8e4&labelColor=232634"></a>
  <a href="https://www.ansible.com"><img alt="Ansible" src="https://img.shields.io/badge/Ansible-f4b8e4?style=flat-square&logo=ansible&logoColor=f4b8e4&labelColor=232634"></a>
  <a href="https://catppuccin.com"><img alt="Catppuccin Latte and Frappe" src="https://img.shields.io/badge/Catppuccin-Latte%20%2B%20Frapp%C3%A9-f4b8e4?style=flat-square&labelColor=ea76cb"></a>
</p>

> [!IMPORTANT]
> This is my personal setup.

This repo is for my machines. It manages:

- Spectrum bootc image on top of `ghcr.io/ublue-os/bluefin-nvidia-open:stable`
- NixOS modules and one host flake configuration under `modules/` and `hosts/`
- Homebrew userland tools from `Brewfile`
- Ansible system/user setup under `ansible/`
- chezmoi dotfiles under `dotfiles/`
- Go helpers, Python/uv automation, Spectrum image build tooling, browser config,
  shells, terminals, editors, desktop glue, and themes

## Linux / Spectrum

Fresh Bluefin install:

```bash
git clone https://github.com/4evy/dotfiles.git ~/dotfiles
cd ~/dotfiles
sudo bootc switch ghcr.io/4evy/spectrum:latest
systemctl reboot
```

After rebooting into Spectrum:

```bash
cd ~/dotfiles
just setup
```

`just install` wraps the same published image switch for machines that already
have `just`:

```text
ghcr.io/4evy/spectrum:latest
```

Use a local image instead:

```bash
just switch
just reboot
```

For quick local image iteration, reuse cached layers:

```bash
just spectrum-dev
```

After changing `spectrum/` or files copied into the image, rebuild and stage the
local image:

```bash
just switch   # rebuild and switch, or stage with bootc upgrade if already tracking it
just reboot
```

Use `just upgrade` to rebuild the local image and run `bootc upgrade` directly.

Use `just update` after the machine is already bootstrapped. It refreshes
Homebrew/Ansible userland, applies chezmoi, then runs host roles. bootc image
updates are handled by `just install`, `just switch`, or `just upgrade`.

Published Spectrum images include OCI build/source/base-image metadata,
`/usr/share/ublue-os/image-info.json`, a digest-bound SPDX SBOM, GitHub build
provenance, and a keyless Cosign signature. Inspect the current OCI labels with:

```bash
docker buildx imagetools inspect ghcr.io/4evy/spectrum:latest \
  --format '{{json .Image.Config.Labels}}'
```

The Artifact Hub labels are complete, but listing the image there remains
opt-in and requires registering `oci://ghcr.io/4evy/spectrum` with Artifact
Hub.

## macOS

```bash
git clone https://github.com/4evy/dotfiles.git ~/dotfiles
cd ~/dotfiles
./ansible/bootstrap.sh ansible/playbooks/userland.yml
chezmoi init --source "$PWD/dotfiles"
chezmoi apply --refresh-externals=auto --force
```

The bootstrap installs Homebrew if needed, installs Ansible tooling, installs
collections, then runs the requested playbook. It installs upstream Nix with
the official [`NixOS/nix-installer`](https://github.com/NixOS/nix-installer)
and adds `deadnix`, `nh`, `nil`, `nom`, `nix-tree`, `nixd`, and `nixfmt` to the
user's Nix profile. Homebrew uses `/opt/homebrew` on Apple Silicon and
`/usr/local` on Intel.

## Nix

The flake exposes:

- `nixosConfigurations.lenovo-legion`
- `nixosModules.default`
- `packages.*.ghidra-mcp`, `ghidra-mcp-headless`, `ghidra-mcp-httpd`,
  `ghidra-mcp-bridge`, `ghidra-mcp-launcher`, `ghidra`, and
  `equicord-settings`
- apps for the `ghidra-mcp` launchers

Useful examples:

```bash
nix flake check
nix run .#ghidra-mcp
sudo nixos-rebuild switch --flake .#lenovo-legion
```

On macOS, install upstream Nix with `NixOS/nix-installer`; on non-Nix Linux
hosts, install Determinate Nix. Both paths ensure the Nix profile tools are
present:

```bash
just nix
```

## Commands

```bash
just                         # list recipes
just doctor setup            # check commands for a workflow profile
just doctor all              # check every known workflow dependency
just status                  # bootc status and image metadata
just reboot                  # reboot through systemd

just install                 # switch to ghcr.io/4evy/spectrum:latest
just build                   # build localhost/spectrum:local
just spectrum-dev            # build localhost/spectrum:local with cache
just switch                  # rebuild and switch/stage local Spectrum image
just upgrade                 # rebuild local Spectrum image and bootc upgrade
just spectrum-lint           # validate Spectrum Python build scripts
just spectrum-boot-report    # report boot/kernel artifact sizes
just spectrum-diff           # compare RPMs in base image vs Spectrum

just setup                   # first full setup after booting Spectrum
just update                  # refresh an already configured machine
just apply                   # apply only chezmoi dotfiles

just smoke                   # build and run Fedora smoke-test container
just smoke-shell             # open the smoke-test container shell
just nix                     # install host Nix and Nix profile tools

just fmt                     # format repo files
just check-format            # check formatting without rewriting files
just lint                    # lint and run project validation
just check                   # alias for lint
just watch check             # rerun a recipe when files change
```

Most recipes have short aliases in `Justfile`: for example `just s`, `just up`,
`just sw`, `just b`, `just f`, `just c`, and `just w`.

## Python automation

Repository-owned workstation and host automation lives in
`ansible/files/scripts/workstation`. Ansible installs the project as an
editable uv tool, exposing the grouped `dotfiles-scripts` CLI and dedicated
commands such as `system-runner`, `install-ghidra-mcp`, and
`hyper-window-tiling-build`.

```bash
uv sync
uv run dotfiles-scripts --help
uv run --locked ruff format --check .
uv run --locked ruff check .
uv run --locked ty check
uv run --locked pytest
```

Install the repository-native uv/Ruff/ty pre-commit gate once per clone:

```bash
just install-hooks
```

The same lock, format, lint, type, test, compile, and package-build gates run
in the Python GitHub Actions workflow. Ruff enables all stable and preview
rules, and ty promotes every rule to an error; the documented exceptions in
`pyproject.toml` are limited to formatter conflicts and repository-specific
command-line or generated-code conventions.

The package shares command, filesystem, HTTP, path, host, validation, and
template helpers from `workstation.lib`. `ansible/bootstrap.sh` remains a
small POSIX bootstrap because it must install Python/uv and Ansible on a fresh
machine; downloaded upstream installer scripts and interactive shell startup
configuration are not repository automation entrypoints.

Keep declarative machine state in Ansible whenever a module exists: package,
service, file, link, and dconf changes belong in roles so check mode and changed
status remain accurate. Python commands are reserved for transformations,
dynamic discovery, application-specific installers, and workflows that Ansible
cannot express directly. The editable uv tool is reinstalled only when
`pyproject.toml` or `uv.lock` changes; normal Python source edits are visible
immediately without rebuilding the tool environment. One-time cleanup stays
out of band: roles and commands describe only the current layout and do not
retain migration branches for retired paths or service names.

## Checks

Run the normal repo checks:

```bash
just check
```

Smoke-test the Fedora container:

```bash
just smoke
just smoke-shell
```

Run direct project checks when debugging one area:

```bash
go test ./...
uv run spectrum-build check
uv run --locked ruff check .
uv run --locked ty check
uv run --locked pytest
cd packages/hyper-window-tiling && bun run check
```

## Ansible

The main playbooks are:

```text
ansible/playbooks/bootstrap.yml   # prerequisites only
ansible/playbooks/userland.yml    # prerequisites, local tools, ecosystem, apps
ansible/playbooks/host.yml        # host-layer scripts/fonts/integrations
ansible/playbooks/site.yml        # userland + host
```

Use the bootstrap wrapper on a fresh Linux or macOS system:

```bash
./ansible/bootstrap.sh ansible/playbooks/userland.yml
```

After dependencies are installed, playbooks can be run directly:

```bash
ansible-playbook ansible/playbooks/host.yml
ansible-playbook ansible/playbooks/site.yml
```

The Ansible/Python boundary is intentionally narrow:

- Ansible core owns facts, files, links, downloads, archives, users/groups,
  systemd units, package repositories, and service state.
- `community.general` owns cross-platform integrations such as Flatpak,
  dconf, Homebrew, launchd, pacman, apk, and kernel module loading.
- The local `evy.dotfiles` collection adapts the remaining dynamic Python
  builders and application-specific workflows to native Ansible results,
  including check mode, changed/skipped state, warnings, diffs, and failures.
- Python remains responsible for computation and transformation work such as
  KMSCON theme rendering, verified upstream builds, release discovery, and
  application-specific configuration formats.

The local collection lives under `ansible/collections`; external collections
are pinned in `ansible/requirements.yml`. Add another collection only when a
role uses one of its modules—`ansible.posix`, `community.sops`, and
`containers.podman` were reviewed but are not dependencies yet because the
current roles do not need their modules.

## Notes

- `just setup` runs userland setup, chezmoi apply, then host roles.
- `just update` is for userland, host integrations, and dotfiles. bootc image
  updates are handled by the image recipes.
- `bootc switch` is for moving to a different image reference. `just switch`
  rebuilds the local Spectrum image, then switches to it or stages it when the
  host is already tracking `localhost/spectrum:local`.
- `just build` defaults to no-cache builds. Use `just spectrum-dev` or
  `just build localhost/spectrum:local false` for cached local iteration.
- Spectrum image defaults can be overridden with `SPECTRUM_IMAGE_NAME`,
  `SPECTRUM_LOCAL_TAG`, `SPECTRUM_REMOTE_REF`, `SPECTRUM_BLUEFIN_BASE_IMAGE`,
  and related environment variables from `Justfile`.
- The published Spectrum image is at
  <https://github.com/4evy/dotfiles/pkgs/container/spectrum>.

## License

[LICENSE.txt](LICENSE.txt)
