#!/usr/bin/env -S just --justfile

set script-interpreter := ['bash', '-euo', 'pipefail']

image_name := env("SPECTRUM_IMAGE_NAME", "spectrum")
local_tag := env("SPECTRUM_LOCAL_TAG", "local")
local_ref := "localhost/" + image_name + ":" + local_tag
remote_ref := env("SPECTRUM_REMOTE_REF", "ghcr.io/4evy/" + image_name + ":latest")
base_image := env("SPECTRUM_BLUEFIN_BASE_IMAGE", "ghcr.io/ublue-os/bluefin-nvidia-open:stable@sha256:1e7a59c83f104652bd06308f0a6439669cb3ea327d4e968695af85c67abea352")
base_image_name := env("SPECTRUM_BLUEFIN_BASE_IMAGE_NAME", "bluefin-nvidia-open")
base_image_tag := env("SPECTRUM_BLUEFIN_BASE_IMAGE_TAG", "stable")
build_no_cache := env("SPECTRUM_BUILD_NO_CACHE", "true")
compose := env("COMPOSE", "podman-compose")
ansible_playbooks := "ansible/playbooks/bootstrap.yml ansible/playbooks/userland.yml ansible/playbooks/host.yml ansible/playbooks/site.yml"
podman := env("PODMAN", "podman")
homebrew_path := "/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/local/sbin"
determinate_nix_installer_url := "https://install.determinate.systems/nix"
nixos_nix_installer_url := "https://artifacts.nixos.org/nix-installer"
nix_bin_dir := "/nix/var/nix/profiles/default/bin"
nix_bin := "/nix/var/nix/profiles/default/bin/nix"
nix_profile_bin_dir := env("HOME", "") + "/.nix-profile/bin"
nixos_profile_bin_dir := "/run/current-system/sw/bin"
nix_profile_tools := "deadnix:deadnix nh:nh nil:nil nix-instantiate:nix nom:nix-output-monitor nix-tree:nix-tree nixd:nixd nixfmt:nixfmt"
export PATH := env("PATH", "") + ":" + nix_bin_dir + ":" + nix_profile_bin_dir + ":" + nixos_profile_bin_dir + ":" + homebrew_path

alias a := apply
alias b := build
alias c := check
alias cf := check-format
alias ck := check
alias f := fmt
alias i := install
alias l := lint
alias nx := nix
alias r := reboot
alias s := setup
alias sw := switch
alias u := upgrade
alias up := update
alias w := watch

[private]
default:
    @just --list --unsorted --list-heading $'Dotfiles recipes:\n'

# Check commands required for a workflow profile.
[arg('profile', pattern='status|reboot|install|build|setup|apply|shell|spectrum|fmt|lint|go|ansible|bun|smoke|nix|watch|check|all', help='status, reboot, install, build, setup, apply, shell, spectrum, fmt, lint, go, ansible, bun, smoke, nix, watch, check, or all')]
[group('system')]
[script]
doctor profile="setup":
    profile={{ quote(profile) }}
    host_os={{ quote(os()) }}
    commands=()
    podman_command={{ quote(podman) }}
    compose_command={{ quote(compose) }}

    case "$profile" in
      status)
        if [[ $host_os == linux ]]; then
          commands=(bootc)
        else
          printf 'Skipping Linux-only dependency check for workflow %q on %s.\n' "$profile" "$host_os"
        fi
        ;;
      reboot)
        if [[ $host_os == linux ]]; then
          commands=(systemctl)
        else
          printf 'Skipping Linux-only dependency check for workflow %q on %s.\n' "$profile" "$host_os"
        fi
        ;;
      install)
        if [[ $host_os == linux ]]; then
          commands=(bootc sudo)
        else
          printf 'Skipping Linux-only dependency check for workflow %q on %s.\n' "$profile" "$host_os"
        fi
        ;;
      build)
        commands=("${podman_command%% *}" sudo)
        ;;
      setup)
        commands=(bash curl git sudo)
        ;;
      apply)
        commands=(chezmoi)
        ;;
      shell)
        commands=(shellcheck shfmt)
        ;;
      spectrum)
        commands=(uv)
        ;;
      fmt)
        commands=(git gofmt go just jq nixfmt prettier shfmt taplo stylua uv)
        if [[ -f packages/hyper-window-tiling/package.json ]]; then
          commands+=(bun)
        fi
        ;;
      lint | check)
        commands=(chezmoi git gofmt go hadolint jq just nix-instantiate nixfmt prettier shellcheck shfmt taplo stylua uv golangci-lint ansible-galaxy ansible-playbook ansible-test ansible-lint actionlint luacheck yamllint)
        if [[ -f packages/hyper-window-tiling/package.json ]]; then
          commands+=(bun)
        fi
        ;;
      go)
        commands=(go golangci-lint)
        ;;
      ansible)
        commands=(ansible-galaxy ansible-playbook ansible-test ansible-lint yamllint)
        ;;
      bun)
        commands=(bun)
        ;;
      smoke)
        commands=("${compose_command%% *}")
        ;;
      nix)
        commands=(bash curl sudo)
        ;;
      watch)
        commands=(watchexec)
        ;;
      all)
        commands=(sudo "${podman_command%% *}" bash chezmoi curl git gofmt hadolint jq just nix-instantiate nixfmt prettier shellcheck shfmt taplo stylua uv go golangci-lint ansible-galaxy ansible-playbook ansible-test ansible-lint actionlint luacheck yamllint watchexec "${compose_command%% *}")
        if [[ $host_os == linux ]]; then
          commands+=(bootc systemctl)
        fi
        if [[ -f packages/hyper-window-tiling/package.json ]]; then
          commands+=(bun)
        fi
        ;;
    esac

    missing=0
    for command in "${commands[@]}"; do
      if ! command -v "$command" >/dev/null 2>&1; then
        printf 'missing command: %s\n' "$command" >&2
        missing=1
      fi
    done
    exit "$missing"

[macos]
[private]
_linux-only recipe:
    @printf 'Skipping `just %s`: this workflow is only supported on Linux.\n' {{ quote(recipe) }}

# Show bootc and image metadata status.
[group('system')]
[linux]
status: (doctor 'status')
    bootc status
    @if [ -r /usr/share/ublue-os/image-info.json ] && command -v jq >/dev/null 2>&1; then \
      jq . /usr/share/ublue-os/image-info.json; \
    fi

[group('system')]
[macos]
status: (_linux-only 'status')

# Reboot the host.
[confirm('Reboot this host now?')]
[group('system')]
[linux]
reboot: (doctor 'reboot')
    systemctl reboot

[group('system')]
[macos]
reboot: (_linux-only 'reboot')

# Build the local Spectrum bootc image. Set no_cache=false to reuse layers.
[arg('target', help='Image reference to tag locally')]
[arg('no_cache', pattern='true|yes|1|on|false|no|0|off', help='true/false, yes/no, 1/0, or on/off')]
[group('spectrum')]
[linux]
[script]
build target=local_ref no_cache=build_no_cache: (doctor 'build')
    target={{ quote(target) }}
    no_cache={{ quote(no_cache) }}
    base_image={{ quote(base_image) }}
    base_image_name={{ quote(base_image_name) }}
    base_image_tag={{ quote(base_image_tag) }}
    image_name={{ quote(image_name) }}
    local_tag={{ quote(local_tag) }}
    read -r -a podman_command <<< {{ quote(podman) }}
    image_created=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    image_revision=$(git rev-parse HEAD 2>/dev/null || printf '%s' unknown)
    image_version=$(git describe --tags --always --dirty 2>/dev/null || printf '%s' "$local_tag")
    base_image_ref=${base_image%@*}
    base_image_digest=${base_image##*@}
    if [[ "$base_image_digest" == "$base_image" ]]; then
      printf '%s\n' 'Spectrum base images must be pinned by digest' >&2
      exit 2
    fi

    build_args=(
      --pull=newer \
      --tag "$target" \
      --build-arg "BLUEFIN_BASE_IMAGE=$base_image" \
      --build-arg "BLUEFIN_BASE_IMAGE_NAME=$base_image_name" \
      --build-arg "BLUEFIN_BASE_IMAGE_TAG=$base_image_tag" \
      --build-arg "IMAGE_NAME=$image_name" \
      --build-arg "IMAGE_TAG=$local_tag" \
      --build-arg "IMAGE_REF=ostree-image:docker://$target" \
      --build-arg "IMAGE_CREATED=$image_created" \
      --build-arg "IMAGE_REVISION=$image_revision" \
      --build-arg "IMAGE_VERSION=$image_version" \
      --label "org.opencontainers.image.base.name=$base_image_ref" \
      --label "org.opencontainers.image.base.digest=$base_image_digest" \
      --file spectrum/Containerfile \
      .
    )

    if [[ "$image_revision" != unknown ]]; then
      repository_url=https://github.com/4evy/dotfiles
      raw_repository_url=https://raw.githubusercontent.com/4evy/dotfiles
      build_args=(
        --build-arg "IMAGE_SOURCE=$repository_url/blob/$image_revision/spectrum/Containerfile"
        --build-arg "IMAGE_URL=$repository_url/tree/$image_revision"
        --build-arg "IMAGE_DOCUMENTATION=$raw_repository_url/$image_revision/README.md"
        --build-arg "IMAGE_README=$raw_repository_url/$image_revision/README.md"
        "${build_args[@]}"
      )
    fi

    case "$no_cache" in
      true | yes | 1 | on)
        build_args=(--no-cache "${build_args[@]}")
        ;;
      false | no | 0 | off)
        ;;
      *)
        printf 'no_cache must be true or false, got: %s\n' "$no_cache" >&2
        exit 2
        ;;
    esac

    github_token_file=
    if [[ -n "${GITHUB_TOKEN:-}" || -n "${GH_TOKEN:-}" ]]; then
      github_token_file=$(mktemp)
      trap 'rm -f "$github_token_file"' EXIT
      printf '%s' "${GITHUB_TOKEN:-$GH_TOKEN}" >"$github_token_file"
      chmod 600 "$github_token_file"
      build_args=(--secret "id=github_token,src=$github_token_file" "${build_args[@]}")
    fi

    sudo env \
      -u XDG_RUNTIME_DIR \
      -u DBUS_SESSION_BUS_ADDRESS \
      -u WAYLAND_DISPLAY \
      -u DISPLAY \
      -u SSH_AUTH_SOCK \
      "${podman_command[@]}" build "${build_args[@]}"

[arg('target', help='Image reference to tag locally')]
[arg('no_cache', pattern='true|yes|1|on|false|no|0|off', help='true/false, yes/no, 1/0, or on/off')]
[group('spectrum')]
[macos]
build target=local_ref no_cache=build_no_cache: (_linux-only 'build')

# Build Spectrum quickly for local iteration, reusing cached layers.
[arg('target', help='Image reference to tag locally')]
[group('spectrum')]
[linux]
spectrum-dev target=local_ref: (build target 'false')

# Validate Spectrum build scripts without building the image.
[group('spectrum')]
spectrum-lint: _check-spectrum

# Report boot and kernel artifact sizes from a built Spectrum image.
[arg('target', help='Built image reference to inspect')]
[group('spectrum')]
[linux]
[script]
spectrum-boot-report target=local_ref: (doctor 'build')
    target={{ quote(target) }}
    read -r -a podman_command <<< {{ quote(podman) }}
    sudo "${podman_command[@]}" run \
      --rm \
      --entrypoint bash \
      --volume "$PWD:/workspace:ro" \
      "$target" \
      -ceu '
        mkdir -p /tmp/spectrum-workspace/spectrum
        cp /workspace/pyproject.toml /workspace/uv.lock /tmp/spectrum-workspace/
        cp -a /workspace/spectrum/scripts /tmp/spectrum-workspace/spectrum/
        UV_PROJECT_ENVIRONMENT=/tmp/spectrum-boot-report-venv \
          uv --cache-dir /tmp/spectrum-uv-cache \
            --directory /tmp/spectrum-workspace \
            run --locked python /tmp/spectrum-workspace/spectrum/scripts/boot_artifacts.py
      '

# Show RPM package differences between the selected Bluefin base and Spectrum.
[arg('base', help='Bluefin base image reference to compare against')]
[arg('target', help='Built Spectrum image reference to inspect')]
[group('spectrum')]
[linux]
[script]
spectrum-diff target=local_ref base=base_image: (doctor 'build')
    target={{ quote(target) }}
    base={{ quote(base) }}
    read -r -a podman_command <<< {{ quote(podman) }}
    work_dir=$(mktemp -d)
    trap 'rm -rf "$work_dir"' EXIT

    sudo "${podman_command[@]}" run --rm --entrypoint rpm "$base" \
      -qa --qf '%{NAME}\n' | sort -u >"$work_dir/base"
    sudo "${podman_command[@]}" run --rm --entrypoint rpm "$target" \
      -qa --qf '%{NAME}\n' | sort -u >"$work_dir/target"

    printf '%s\n' 'Only in Spectrum:'
    comm -13 "$work_dir/base" "$work_dir/target" || true
    printf '\n%s\n' 'Only in base image:'
    comm -23 "$work_dir/base" "$work_dir/target" || true

# Switch the host to the published Spectrum image on next boot.
[arg('target', help='Published bootc image reference')]
[confirm('Switch this host to published Spectrum image ' + target + '?')]
[group('spectrum')]
[linux]
install target=remote_ref: (doctor 'install')
    sudo bootc switch {{ quote(target) }}
    @printf '%s\n' 'Run `just reboot`, then `just setup` after reboot.'

[arg('target', help='Published bootc image reference')]
[group('spectrum')]
[macos]
install target=remote_ref: (_linux-only 'install')

# Rebuild and switch the host to the local Spectrum image on next boot.
[arg('target', help='Local containers-storage image reference')]
[confirm('Switch this host to local Spectrum image ' + target + '?')]
[group('spectrum')]
[linux]
[script]
switch target=local_ref: (doctor 'install') (build target)
    target={{ quote(target) }}
    switch_log=$(mktemp)
    trap 'rm -f "$switch_log"' EXIT

    if sudo bootc switch --transport containers-storage "$target" \
      > >(tee "$switch_log") \
      2> >(tee -a "$switch_log" >&2); then
      switch_status=0
    else
      switch_status=$?
    fi

    if grep -Fxq 'Image specification is unchanged.' "$switch_log"; then
      printf 'Already tracking %s; staging the latest local image with `bootc upgrade`.\n' "$target"
      sudo bootc upgrade
      exit 0
    fi

    exit "$switch_status"

[arg('target', help='Local containers-storage image reference')]
[group('spectrum')]
[macos]
switch target=local_ref: (_linux-only 'switch')

# Rebuild the local Spectrum image and stage it as an upgrade.
[arg('target', help='Local image reference to rebuild before upgrade')]
[confirm('Rebuild ' + target + ' and stage it as a bootc upgrade?')]
[group('spectrum')]
[linux]
upgrade target=local_ref: (doctor 'install') (build target)
    sudo bootc upgrade

[arg('target', help='Local image reference to rebuild before upgrade')]
[group('spectrum')]
[macos]
upgrade target=local_ref: (_linux-only 'upgrade')

# Build the Fedora smoke-test image and run its default validation command.
[group('containers')]
[script]
smoke: (doctor 'smoke')
    read -r -a compose_command <<< {{ quote(compose) }}
    "${compose_command[@]}" build
    "${compose_command[@]}" run --rm fedora

# Open an interactive shell in the Fedora smoke-test image.
[group('containers')]
[script]
smoke-shell: (doctor 'smoke')
    read -r -a compose_command <<< {{ quote(compose) }}
    "${compose_command[@]}" run --rm fedora-shell

# Install Nix on the live host and ensure Nix profile tools exist.
[group('setup')]
[script]
nix: (doctor 'nix')
    kernel_name=$(uname -s)

    if [[ $kernel_name == Linux && ! -e /nix ]] &&
      command -v rpm-ostree >/dev/null 2>&1 &&
      findmnt --noheadings --output SOURCE,FSTYPE,OPTIONS / | grep -Eq '(^|[[:space:]])composefs([[:space:]]|$)'; then
      printf '%s\n' 'This composefs ostree host needs /nix in the booted image before installing Nix.' >&2
      printf '%s\n' 'Rebuild and boot Spectrum with the /nix mountpoint, then rerun just nix.' >&2
      exit 1
    fi

    export PATH="{{ nix_bin_dir }}:$HOME/.nix-profile/bin:$PATH"

    if ! command -v nix >/dev/null 2>&1; then
      case "$kernel_name" in
        Darwin)
          installer_url={{ quote(nixos_nix_installer_url) }}
          install_args=(install macos --enable-flakes --no-confirm --no-modify-profile)
          ;;
        Linux)
          installer_url={{ quote(determinate_nix_installer_url) }}
          plan=linux
          if command -v rpm-ostree >/dev/null 2>&1; then
            plan=ostree
          fi
          install_args=(install "$plan" --no-confirm)
          ;;
        *)
          printf 'unsupported operating system for Nix installation: %s\n' "$kernel_name" >&2
          exit 2
          ;;
      esac
      curl -fsSL "$installer_url" | sh -s -- "${install_args[@]}"
    fi

    nix_bin={{ quote(nix_bin) }}
    if [[ ! -x $nix_bin ]]; then
      nix_bin=$(command -v nix)
    fi

    missing=()
    for spec in {{ nix_profile_tools }}; do
      bin=${spec%%:*}
      pkg=${spec#*:}
      if ! command -v "$bin" >/dev/null 2>&1; then
        missing+=("nixpkgs#$pkg")
      fi
    done

    if ((${#missing[@]})); then
      "$nix_bin" profile install "${missing[@]}"
    else
      printf '%s\n' 'Nix profile tools already installed.'
    fi

# Apply chezmoi-managed dotfiles.
[group('setup')]
apply: (doctor 'apply')
    PATH="{{ homebrew_path }}:$PATH" chezmoi init --source "$PWD/dotfiles"
    PATH="{{ homebrew_path }}:$PATH" chezmoi apply --refresh-externals=auto --force

[doc('Bootstrap userland, apply dotfiles, then apply host roles.')]
[group('setup')]
setup: (doctor 'setup') _userland apply _host

# Refresh userland, dotfiles, and host roles on an already-bootstrapped machine.
[group('setup')]
update: _userland apply _host

[private]
[script]
_deps:
    install_args=(collection install -r ansible/requirements.yml -p .ansible/collections)
    if [[ ! -f .ansible/collections/ansible_collections/community/general/MANIFEST.json ]]; then
      install_args+=(--force)
    fi
    PATH="{{ homebrew_path }}:$PATH" ansible-galaxy "${install_args[@]}"

[private]
_userland:
    PATH="{{ homebrew_path }}:$PATH" ./ansible/bootstrap.sh ansible/playbooks/userland.yml

[private]
_host:
    PATH="{{ homebrew_path }}:$PATH" ansible-playbook ansible/playbooks/host.yml

# Format files managed by this repo.
[group('dev')]
fmt: (_format-files 'write') (_hyper-format 'write')

# Rerun a recipe when files change.
[arg('args', help='Recipe and arguments to rerun on file changes')]
[group('dev')]
[positional-arguments]
watch +args='check': (doctor 'watch')
    watchexec --clear --restart -- just "$@"

# Check repository formatting without rewriting files.
[group('dev')]
check-format: (_format-files 'check') (_hyper-format 'check')

[private]
[script]
_format-files mode: (doctor 'fmt')
    mode={{ quote(mode) }}

    case "$mode" in
      write | check)
        ;;
      *)
        printf 'unknown format mode: %s\n' "$mode" >&2
        exit 2
        ;;
    esac

    repo_paths=()
    missing_paths=()
    symlink_files=()
    special_files=()
    template_files=()
    unformatted_files=()
    hyper_files=()
    go_files=()
    go_mod_files=()
    json_files=()
    jsonc_files=()
    prettier_files=()
    python_files=()
    shell_files=()
    toml_files=()
    lua_files=()
    nix_files=()
    just_files=()

    is_shell_file() {
      case "$1" in
        *.tmpl | *.txt | *.patch)
          return 1
          ;;
        *.sh | *.sh.in | *.bash)
          return 0
          ;;
      esac

      head -n 2 -- "$1" | grep -Eq '^#!.*\b(sh|bash)\b|^# shellcheck shell=(sh|bash)'
    }

    classify_file() {
      local file=$1
      local base=${file##*/}

      case "$file" in
        packages/hyper-window-tiling/*)
          hyper_files+=("$file")
          return
          ;;
      esac

      case "$base" in
        Justfile | justfile)
          just_files+=("$file")
          return
          ;;
        go.mod)
          go_mod_files+=("$file")
          return
          ;;
        flake.lock)
          json_files+=("$file")
          return
          ;;
        uv.lock)
          unformatted_files+=("$file")
          return
          ;;
      esac

      case "$file" in
        *.tmpl)
          template_files+=("$file")
          ;;
        *.jsonc)
          jsonc_files+=("$file")
          ;;
        *.json)
          json_files+=("$file")
          ;;
        *.yaml | *.yml | *.md)
          prettier_files+=("$file")
          ;;
        *.js | *.cjs | *.mjs | *.jsx | *.ts | *.tsx | *.css | *.html | *.mdx)
          prettier_files+=("$file")
          ;;
        *.go)
          go_files+=("$file")
          ;;
        *.py)
          python_files+=("$file")
          ;;
        *.sh | *.sh.in | *.bash)
          shell_files+=("$file")
          ;;
        *.toml)
          toml_files+=("$file")
          ;;
        *.lua)
          lua_files+=("$file")
          ;;
        *.nix)
          nix_files+=("$file")
          ;;
        .envrc | go.sum | *.lock | *.sum | *.txt | *.conf | *.cfg | *.ini | *.service | *.path | *.desktop | *.rules | *.repo | *.plist | *.xml | *.kbd | *.te | *.fc | *.if | *.patch | *.gitattributes | *.gitignore | *.dockerignore | *.shellcheckrc | *.yamllint | *.ansible-lint | *.chezmoiignore | *.chezmoiremove | Brewfile | Dockerfile | Containerfile)
          if is_shell_file "$file"; then
            shell_files+=("$file")
          else
            unformatted_files+=("$file")
          fi
          ;;
        *)
          if is_shell_file "$file"; then
            shell_files+=("$file")
          else
            unformatted_files+=("$file")
          fi
          ;;
      esac
    }

    while IFS= read -r -d '' file; do
      repo_paths+=("$file")
      if [[ -L "$file" ]]; then
        symlink_files+=("$file")
      elif [[ ! -e "$file" ]]; then
        missing_paths+=("$file")
      elif [[ -f "$file" ]]; then
        classify_file "$file"
      else
        special_files+=("$file")
      fi
    done < <(git ls-files -z --cached --others --exclude-standard)

    formatted_count=$((${#hyper_files[@]} + ${#go_files[@]} + ${#go_mod_files[@]} + ${#json_files[@]} + ${#jsonc_files[@]} + ${#prettier_files[@]} + ${#python_files[@]} + ${#shell_files[@]} + ${#toml_files[@]} + ${#lua_files[@]} + ${#nix_files[@]} + ${#just_files[@]}))
    accounted_count=$((formatted_count + ${#template_files[@]} + ${#unformatted_files[@]} + ${#symlink_files[@]} + ${#missing_paths[@]} + ${#special_files[@]}))
    if ((accounted_count != ${#repo_paths[@]})); then
      printf 'internal error: classified %d of %d repository paths\n' "$accounted_count" "${#repo_paths[@]}" >&2
      exit 1
    fi

    printf 'repo paths: %d; formatting check/write covers %d content files; templates enumerated: %d; no structured formatter: %d; symlinks: %d; special: %d; missing/deleted: %d\n' \
      "${#repo_paths[@]}" \
      "$formatted_count" \
      "${#template_files[@]}" \
      "${#unformatted_files[@]}" \
      "${#symlink_files[@]}" \
      "${#special_files[@]}" \
      "${#missing_paths[@]}"

    if ((${#missing_paths[@]} > 0)); then
      printf 'tracked paths missing from worktree:\n' >&2
      printf '  %s\n' "${missing_paths[@]}" >&2
    fi

    missing=()
    require_command() {
      if ! command -v "$1" >/dev/null 2>&1; then
        missing+=("$1")
      fi
    }

    require_command jq
    require_command just
    ((${#go_files[@]} == 0 && ${#go_mod_files[@]} == 0)) || require_command go
    ((${#go_files[@]} == 0)) || require_command gofmt
    ((${#python_files[@]} == 0)) || require_command uv
    if ((${#json_files[@]} > 0 || ${#jsonc_files[@]} > 0 || ${#prettier_files[@]} > 0)); then
      require_command prettier
    fi
    ((${#shell_files[@]} == 0)) || require_command shfmt
    ((${#toml_files[@]} == 0)) || require_command taplo
    ((${#lua_files[@]} == 0)) || require_command stylua
    if [[ -f packages/hyper-window-tiling/package.json ]]; then
      require_command bun
    fi

    nix_formatter=()
    if ((${#nix_files[@]} > 0)); then
      if command -v nixfmt >/dev/null 2>&1; then
        if [[ "$mode" == write ]]; then
          nix_formatter=(nixfmt)
        else
          nix_formatter=(nixfmt --check)
        fi
      else
        missing+=("nixfmt")
      fi
    fi

    if ((${#missing[@]} > 0)); then
      printf 'missing formatter command: %s\n' "${missing[@]}" >&2
      exit 1
    fi

    check_gofmt() {
      local gofmt_output
      if ! gofmt_output=$(gofmt -l "$@"); then
        printf '%s\n' 'gofmt failed' >&2
        exit 1
      fi
      if [[ -n "$gofmt_output" ]]; then
        printf '%s\n' "$gofmt_output"
        printf '%s\n' 'Go files need gofmt' >&2
        exit 1
      fi
    }

    check_go_mod_fmt() {
      local file tmp
      for file in "$@"; do
        tmp=$(mktemp)
        cp -- "$file" "$tmp"
        go mod edit -fmt "$tmp"
        if ! cmp -s -- "$file" "$tmp"; then
          printf '%s\n' "$file"
          printf '%s\n' 'go.mod files need go mod edit -fmt' >&2
          rm -f -- "$tmp"
          exit 1
        fi
        rm -f -- "$tmp"
      done
    }

    ((${#json_files[@]} == 0)) || jq empty "${json_files[@]}"

    if [[ "$mode" == write ]]; then
      ((${#just_files[@]} == 0)) || just --fmt -f Justfile
      ((${#go_files[@]} == 0)) || gofmt -w "${go_files[@]}"
      for file in "${go_mod_files[@]}"; do
        go mod edit -fmt "$file"
      done
      ((${#python_files[@]} == 0)) || uv run --locked ruff format --force-exclude "${python_files[@]}"
      ((${#json_files[@]} == 0)) || prettier --write --parser json "${json_files[@]}"
      ((${#jsonc_files[@]} == 0)) || prettier --write --parser jsonc --trailing-comma none "${jsonc_files[@]}"
      ((${#prettier_files[@]} == 0)) || prettier --write "${prettier_files[@]}"
      ((${#shell_files[@]} == 0)) || shfmt -ci -w "${shell_files[@]}"
      ((${#toml_files[@]} == 0)) || taplo fmt "${toml_files[@]}"
      ((${#lua_files[@]} == 0)) || stylua "${lua_files[@]}"
      ((${#nix_files[@]} == 0)) || "${nix_formatter[@]}" "${nix_files[@]}"
    else
      ((${#just_files[@]} == 0)) || just --fmt --check -f Justfile
      ((${#go_files[@]} == 0)) || check_gofmt "${go_files[@]}"
      ((${#go_mod_files[@]} == 0)) || check_go_mod_fmt "${go_mod_files[@]}"
      ((${#python_files[@]} == 0)) || uv run --locked ruff format --check --force-exclude "${python_files[@]}"
      ((${#json_files[@]} == 0)) || prettier --check --parser json "${json_files[@]}"
      ((${#jsonc_files[@]} == 0)) || prettier --check --parser jsonc --trailing-comma none "${jsonc_files[@]}"
      ((${#prettier_files[@]} == 0)) || prettier --check "${prettier_files[@]}"
      ((${#shell_files[@]} == 0)) || shfmt -ci -d "${shell_files[@]}"
      ((${#toml_files[@]} == 0)) || taplo fmt --check "${toml_files[@]}"
      ((${#lua_files[@]} == 0)) || stylua --check "${lua_files[@]}"
      ((${#nix_files[@]} == 0)) || "${nix_formatter[@]}" "${nix_files[@]}"
    fi

[private]
[script]
[working-directory('packages/hyper-window-tiling')]
_hyper-format mode: (doctor 'bun')
    mode={{ quote(mode) }}
    case "$mode" in
      write)
        bun run format
        ;;
      check)
        bun run biome format .
        ;;
      *)
        printf 'unknown hyper format mode: %s\n' "$mode" >&2
        exit 2
        ;;
    esac

[private]
[script]
_lint-files: (doctor 'lint')
    repo_paths=()
    missing_paths=()
    symlink_files=()
    broken_symlinks=()
    template_files=()
    python_template_files=()
    python_input_template_files=()
    shell_template_files=()
    xml_input_template_files=()
    docker_files=()
    json_files=()
    jsonc_files=()
    yaml_files=()
    toml_files=()
    xml_files=()
    python_files=()
    shell_files=()
    nix_files=()
    lua_files=()
    go_files=()
    go_mod_files=()
    prettier_parse_files=()
    hyper_files=()
    enumerated_files=()

    is_shell_file() {
      case "$1" in
        *.tmpl | *.txt | *.patch)
          return 1
          ;;
        *.sh | *.sh.in | *.bash)
          return 0
          ;;
      esac

      head -n 2 -- "$1" | grep -Eq '^#!.*\b(sh|bash)\b|^# shellcheck shell=(sh|bash)'
    }

    classify_file() {
      local file=$1
      local base=${file##*/}

      case "$file" in
        packages/hyper-window-tiling/*)
          hyper_files+=("$file")
          return
          ;;
      esac

      case "$base" in
        Justfile | justfile)
          enumerated_files+=("$file")
          return
          ;;
        Dockerfile | Containerfile)
          docker_files+=("$file")
          return
          ;;
        go.mod)
          go_mod_files+=("$file")
          return
          ;;
        flake.lock)
          json_files+=("$file")
          return
          ;;
        uv.lock)
          enumerated_files+=("$file")
          return
          ;;
      esac

      case "$file" in
        *.plist.in | *.xml.in)
          xml_input_template_files+=("$file")
          ;;
        *.py.in)
          python_input_template_files+=("$file")
          ;;
        *.bash.tmpl | *.sh.tmpl)
          template_files+=("$file")
          shell_template_files+=("$file")
          ;;
        *.py.tmpl)
          template_files+=("$file")
          python_template_files+=("$file")
          ;;
        *.tmpl)
          template_files+=("$file")
          ;;
        *.jsonc)
          jsonc_files+=("$file")
          ;;
        *.json)
          json_files+=("$file")
          ;;
        *.yaml | *.yml | .yamllint | .ansible-lint)
          yaml_files+=("$file")
          ;;
        *.toml)
          toml_files+=("$file")
          ;;
        *.plist | *.xml)
          xml_files+=("$file")
          ;;
        *.py)
          python_files+=("$file")
          ;;
        *.sh | *.sh.in | *.bash)
          shell_files+=("$file")
          ;;
        *.nix)
          nix_files+=("$file")
          ;;
        *.lua)
          lua_files+=("$file")
          ;;
        *.go)
          go_files+=("$file")
          ;;
        *.md | *.js | *.cjs | *.mjs | *.jsx | *.ts | *.tsx | *.css | *.html | *.mdx)
          prettier_parse_files+=("$file")
          ;;
        .envrc)
          shell_files+=("$file")
          ;;
        *)
          if is_shell_file "$file"; then
            shell_files+=("$file")
          else
            enumerated_files+=("$file")
          fi
          ;;
      esac
    }

    while IFS= read -r -d '' file; do
      repo_paths+=("$file")
      if [[ -L "$file" ]]; then
        symlink_files+=("$file")
        [[ -e "$file" ]] || broken_symlinks+=("$file")
      elif [[ ! -e "$file" ]]; then
        missing_paths+=("$file")
      elif [[ -f "$file" ]]; then
        classify_file "$file"
      else
        enumerated_files+=("$file")
      fi
    done < <(git ls-files -z --cached --others --exclude-standard)

    linted_count=$((${#hyper_files[@]} + ${#docker_files[@]} + ${#go_files[@]} + ${#go_mod_files[@]} + ${#json_files[@]} + ${#jsonc_files[@]} + ${#yaml_files[@]} + ${#toml_files[@]} + ${#xml_files[@]} + ${#python_files[@]} + ${#shell_files[@]} + ${#nix_files[@]} + ${#lua_files[@]} + ${#prettier_parse_files[@]}))
    accounted_count=$((linted_count + ${#template_files[@]} + ${#python_input_template_files[@]} + ${#xml_input_template_files[@]} + ${#enumerated_files[@]} + ${#symlink_files[@]} + ${#missing_paths[@]}))
    if ((accounted_count != ${#repo_paths[@]})); then
      printf 'internal error: classified %d of %d repository paths\n' "$accounted_count" "${#repo_paths[@]}" >&2
      exit 1
    fi

    printf 'repo paths: %d; linted/parsed content files: %d; chezmoi templates: %d (python syntax: %d; shell syntax: %d); generated syntax: %d python, %d XML; enumerated-only files: %d; symlinks: %d; missing/deleted: %d; broken symlinks: %d\n' \
      "${#repo_paths[@]}" \
      "$linted_count" \
      "${#template_files[@]}" \
      "${#python_template_files[@]}" \
      "${#shell_template_files[@]}" \
      "${#python_input_template_files[@]}" \
      "${#xml_input_template_files[@]}" \
      "${#enumerated_files[@]}" \
      "${#symlink_files[@]}" \
      "${#missing_paths[@]}" \
      "${#broken_symlinks[@]}"

    if ((${#missing_paths[@]} > 0)); then
      printf 'tracked paths missing from worktree:\n' >&2
      printf '  %s\n' "${missing_paths[@]}" >&2
    fi

    if ((${#broken_symlinks[@]} > 0)); then
      printf 'broken symlinks:\n' >&2
      printf '  %s\n' "${broken_symlinks[@]}" >&2
      exit 1
    fi

    check_xml() {
      uv run --locked python - "$@" <<'PY'
    from __future__ import annotations

    import sys

    from defusedxml.ElementTree import parse

    for path in sys.argv[1:]:
        try:
            parse(path)
        except Exception as error:
            raise SystemExit(f"{path}: {error}") from error
    PY
    }

    ((${#yaml_files[@]} == 0)) || yamllint "${yaml_files[@]}"
    ((${#docker_files[@]} == 0)) || hadolint "${docker_files[@]}"
    ((${#toml_files[@]} == 0)) || taplo lint "${toml_files[@]}"
    ((${#xml_files[@]} == 0)) || check_xml "${xml_files[@]}"
    ((${#python_files[@]} == 0)) || uv run --locked ruff check --force-exclude "${python_files[@]}"
    ((${#shell_files[@]} == 0)) || shellcheck -x "${shell_files[@]}"
    ((${#nix_files[@]} == 0)) || for file in "${nix_files[@]}"; do nix-instantiate --parse "$file" >/dev/null; done
    ((${#lua_files[@]} == 0)) || luacheck --globals Command cx ya -- "${lua_files[@]}"

    if ((${#template_files[@]} > 0 || ${#python_input_template_files[@]} > 0 || ${#xml_input_template_files[@]} > 0)); then
      tmp_destination=$(mktemp -d)
      trap 'rm -rf "$tmp_destination"' EXIT
      if ((${#template_files[@]} > 0)); then
        chezmoi apply \
          --dry-run \
          --source "$PWD/dotfiles" \
          --destination "$tmp_destination" \
          --force \
          --no-tty \
          --refresh-externals=never >/dev/null
      fi

      for file in "${python_template_files[@]}" "${shell_template_files[@]}"; do
        rendered_file="$tmp_destination/rendered-templates/${file%.tmpl}"
        mkdir -p "${rendered_file%/*}"
        chezmoi --source "$PWD/dotfiles" execute-template < "$file" > "$rendered_file"
      done
      if ((${#python_template_files[@]} > 0)); then
        PYTHONPYCACHEPREFIX="$tmp_destination/pycache" \
          uv run --locked python -m compileall -q "$tmp_destination/rendered-templates"
        for file in "${python_template_files[@]}"; do
          rendered_file="$tmp_destination/rendered-templates/${file%.tmpl}"
          uv run --locked ruff check --stdin-filename "${file%.tmpl}" - < "$rendered_file"
        done
      fi
      for file in "${shell_template_files[@]}"; do
        rendered_file="$tmp_destination/rendered-templates/${file%.tmpl}"
        bash -n "$rendered_file"
      done
      for file in "${python_input_template_files[@]}"; do
        rendered_file="$tmp_destination/rendered-input-templates/${file%.in}"
        mkdir -p "${rendered_file%/*}"
        sed -E 's/@[A-Za-z_][A-Za-z0-9_]*@/template_value/g' "$file" > "$rendered_file"
      done
      if ((${#python_input_template_files[@]} > 0)); then
        PYTHONPYCACHEPREFIX="$tmp_destination/pycache" \
          uv run --locked python -m compileall -q "$tmp_destination/rendered-input-templates"
      fi
      rendered_xml_files=()
      for file in "${xml_input_template_files[@]}"; do
        rendered_file="$tmp_destination/rendered-input-templates/${file%.in}"
        mkdir -p "${rendered_file%/*}"
        sed -E 's/@[A-Za-z_][A-Za-z0-9_]*@/template_value/g' "$file" > "$rendered_file"
        rendered_xml_files+=("$rendered_file")
      done
      ((${#rendered_xml_files[@]} == 0)) || check_xml "${rendered_xml_files[@]}"
    fi

[private]
_check-spectrum-build: (doctor 'spectrum')
    uv run --locked spectrum-build check

[private]
[script]
_check-spectrum: (doctor 'spectrum') _check-spectrum-build
    bytecode_dir=$(mktemp -d)
    trap 'rm -rf "$bytecode_dir"' EXIT
    uv run --locked ty check spectrum/scripts/build.py spectrum/scripts/boot_artifacts.py spectrum/scripts/spectrum_build
    PYTHONPYCACHEPREFIX="$bytecode_dir" uv run --locked python -m compileall -q spectrum/scripts

[private]
[script]
_check-python:
    uv lock --check
    uv sync --locked --check
    uv run --locked ty check
    bytecode_dir=$(mktemp -d)
    build_dir=$(mktemp -d)
    trap 'rm -rf "$bytecode_dir" "$build_dir"' EXIT
    PYTHONPYCACHEPREFIX="$bytecode_dir" uv run --locked python -m compileall -q ansible/files/scripts dotfiles/.chezmoiscripts internal/chromiumbrowser/scripts packages/toshy spectrum/scripts
    uv run --locked pytest
    uv build --out-dir "$build_dir" --no-build-logs

[private]
_check-go: (doctor 'go')
    go test ./...
    golangci-lint run ./...

[private]
[script]
_check-ansible: (doctor 'ansible') _deps _check-ansible-collection
    for playbook in {{ ansible_playbooks }}; do
      ansible-playbook --syntax-check "$playbook"
    done
    ansible-lint ansible
    yamllint .

[private]
[working-directory('ansible/collections/ansible_collections/evy/dotfiles')]
_check-ansible-collection:
    ansible-test sanity --local --skip-test validate-modules
    ansible-test integration --local operation

[private]
_check-github-actions:
    actionlint

[private]
[script]
[working-directory('packages/hyper-window-tiling')]
_check-bun: (doctor 'bun')
    bun install --frozen-lockfile
    bun run check

# Lint repository source files and run project validation.
[group('dev')]
lint: (doctor 'lint') check-format _lint-files _check-python _check-spectrum-build _check-go _check-ansible _check-github-actions _check-bun

# Run the repo validation suite.
[group('dev')]
check: lint
