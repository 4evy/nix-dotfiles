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
- Go helpers, Spectrum image build tooling, local scripts, browser config,
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

## macOS

```bash
git clone https://github.com/4evy/dotfiles.git ~/dotfiles
cd ~/dotfiles
./ansible/bootstrap.sh ansible/playbooks/userland.yml
chezmoi init --source "$PWD/dotfiles"
chezmoi apply --refresh-externals=always --force
```

The bootstrap installs Homebrew if needed, installs Ansible tooling, installs
collections, then runs the requested playbook. Homebrew uses `/opt/homebrew` on
Apple Silicon and `/usr/local` on Intel.

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

On non-Nix hosts, build the Nix distrobox and export the Nix tools:

```bash
just nix
just nix-in
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
just nix                     # build/create the Nix distrobox
just nix-in                  # enter the Nix distrobox

just fmt                     # format repo files
just check-format            # check formatting without rewriting files
just lint                    # lint and run project validation
just check                   # alias for lint
just watch check             # rerun a recipe when files change
```

Most recipes have short aliases in `Justfile`: for example `just s`, `just up`,
`just sw`, `just b`, `just f`, `just c`, and `just w`.

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
uv run ty check spectrum/scripts/build.py spectrum/scripts/boot_artifacts.py spectrum/scripts/spectrum_build
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
