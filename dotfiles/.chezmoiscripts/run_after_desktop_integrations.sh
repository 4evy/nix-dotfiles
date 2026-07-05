#!/bin/sh
set -eu

find_repo_dir() {
	dir=$1
	while :; do
		if [ -f "$dir/go.mod" ] && [ -f "$dir/ansible/playbooks/host.yml" ]; then
			printf '%s\n' "$dir"
			return 0
		fi
		next=${dir%/*}
		if [ "$next" = "$dir" ]; then
			return 1
		fi
		dir=$next
	done
}

source_dir=${CHEZMOI_SOURCE_DIR:-"$HOME/nix-dotfiles/dotfiles"}
if ! repo_dir=$(find_repo_dir "$source_dir"); then
	printf 'chezmoi desktop integrations skipped: could not find repo root from %s\n' "$source_dir" >&2
	exit 0
fi

ensure_host_sudo() {
	reason=$1
	runner=${HOME}/.local/bin/system-runner
	if [ ! -x "$runner" ]; then
		runner=${HOME}/.local/opt/system-run-mcp/latest/bin/system-runner
	fi
	if [ ! -x "$runner" ]; then
		printf '%s\n' "$reason skipped: system-runner was not found" >&2
		return 1
	fi

	if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then
		if ! command -v distrobox-host-exec >/dev/null 2>&1; then
			printf '%s\n' "$reason skipped: distrobox-host-exec was not found" >&2
			return 1
		fi

		if distrobox-host-exec sudo -n "$runner" true >/dev/null 2>&1; then
			return 0
		fi
	else
		if ! command -v sudo >/dev/null 2>&1; then
			printf '%s\n' "$reason skipped: sudo was not found" >&2
			return 1
		fi

		if sudo -n "$runner" true >/dev/null 2>&1; then
			return 0
		fi
	fi

	printf '%s\n' "$reason skipped: passwordless sudo for system-runner is not available" >&2
	return 1
}

ensure_chezmoi_support() {
	bin_dir=${HOME}/.local/bin
	helper=${bin_dir}/chezmoi-support
	builder=${repo_dir}/ansible/files/scripts/local/hyper-window-tiling-build.sh

	if command -v hyper-window-tiling-build >/dev/null 2>&1; then
		return 0
	fi
	if [ -x "${bin_dir}/hyper-window-tiling-build" ]; then
		return 0
	fi
	if [ -x "$builder" ]; then
		return 0
	fi

	if command -v chezmoi-support >/dev/null 2>&1 &&
		chezmoi-support hyper-window-tiling-build --help >/dev/null 2>&1; then
		return 0
	fi
	if [ -x "$helper" ] && "$helper" hyper-window-tiling-build --help >/dev/null 2>&1; then
		return 0
	fi

	if ! command -v go >/dev/null 2>&1; then
		printf '%s\n' "chezmoi desktop integrations require go to build chezmoi-support" >&2
		exit 1
	fi

	mkdir -p "$bin_dir"
	(
		cd "$repo_dir"
		GOBIN="$bin_dir" go install ./cmd/chezmoi-support
	)

	if ! "$helper" hyper-window-tiling-build --help >/dev/null 2>&1; then
		printf '%s\n' "failed to install chezmoi-support with hyper-window-tiling-build support" >&2
		exit 1
	fi
}

if command -v ansible-playbook >/dev/null 2>&1; then
	ansible_playbook=$(command -v ansible-playbook)
	set -- "$ansible_playbook"
elif command -v uvx >/dev/null 2>&1; then
	uvx_bin=$(command -v uvx)
	set -- "$uvx_bin" --from ansible-core ansible-playbook
elif [ -x "$HOME/.local/bin/uvx" ]; then
	set -- "$HOME/.local/bin/uvx" --from ansible-core ansible-playbook
else
	printf '%s\n' "chezmoi desktop integrations skipped: ansible-playbook/uvx not found" >&2
	exit 0
fi

cd "$repo_dir"
ensure_chezmoi_support

exec "$@" ansible/playbooks/host.yml \
	--tags always,hyper-window-tiling,sushi-preview,telegram-flatpak,emoji-shortcut
