#!/usr/bin/env bash
# shellcheck shell=bash
## usage: install-ghostty-tip-linux.sh <cache-dir> <install-prefix>
##
## Builds the upstream Ghostty tip source tarball in a throwaway Podman Fedora
## container, then installs the native release build into <install-prefix>.
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)

# shellcheck source=ansible/files/scripts/host/lib/entrypoint.sh
source -p "$script_dir/../host/lib" entrypoint.sh
source_host_lib fs

usage() {
	sed -n 's/^##[[:space:]]\{0,1\}//p' "${BASH_SOURCE[0]}"
}

if [[ ${1:-} == "-h" || ${1:-} == "--help" ]]; then
	usage
	exit 0
fi

if (($# != 2)); then
	usage >&2
	exit 2
fi

cache_dir=$1
install_prefix=$2
repo=ghostty-org/ghostty
zig_version=0.15.2
container_image=${GHOSTTY_BUILD_CONTAINER_IMAGE:-registry.fedoraproject.org/fedora:latest}
check_interval_seconds=${GHOSTTY_TIP_CHECK_INTERVAL_SECONDS:-86400}
state_version=1
ghostty_bin="${install_prefix%/}/bin/ghostty"
checked_at_file="${install_prefix%/}/.ghostty-tip-checked-at"
source_key_file="${install_prefix%/}/.ghostty-tip-source-key"
state_version_file="${install_prefix%/}/.ghostty-tip-state-version"

missing_runtime_libraries() {
	[[ -x $ghostty_bin ]] || return 0
	LC_ALL=C ldd "$ghostty_bin" 2>/dev/null | awk '/=> not found$/ { print $1 }'
}

verify_runtime_libraries() {
	local missing

	missing=$(missing_runtime_libraries)
	if [[ -z $missing ]]; then
		return 0
	fi

	printf 'error: Ghostty is missing runtime libraries:\n' >&2
	printf '  %s\n' "$missing" >&2
	if grep -Fxq 'libgtk4-layer-shell.so.0' <<<"$missing"; then
		printf '%s\n' 'Add gtk4-layer-shell to the Spectrum image, boot into it, and retry.' >&2
	fi
	return 1
}

zig_arch() {
	case "$(uname -m)" in
		x86_64) printf '%s\n' x86_64-linux ;;
		aarch64 | arm64) printf '%s\n' aarch64-linux ;;
		*)
			printf 'error: unsupported architecture for Ghostty build: %s\n' "$(uname -m)" >&2
			exit 1
			;;
	esac
}

cleanup() {
	if [[ -n ${work_dir:-} ]]; then
		remove_path "$work_dir"
	fi
}

is_nonnegative_integer() {
	[[ ${1:-} =~ ^[0-9]+$ ]]
}

now_seconds() {
	date +%s
}

fresh_check_exists() {
	local checked_at now

	[[ -x $ghostty_bin ]] || return 1
	[[ -z $(missing_runtime_libraries) ]] || return 1
	[[ -r $checked_at_file ]] || return 1
	[[ -r $source_key_file ]] || return 1
	[[ -r $state_version_file ]] || return 1
	[[ $(<"$state_version_file") == "$state_version" ]] || return 1
	is_nonnegative_integer "$check_interval_seconds" || return 1

	checked_at=$(<"$checked_at_file")
	is_nonnegative_integer "$checked_at" || return 1
	now=$(now_seconds)
	((now - checked_at < check_interval_seconds))
}

rewrite_ghostty_desktop_file() {
	local path=${1:?desktop file is required}

	sed \
		-e "s|^TryExec=.*|TryExec=${ghostty_bin}|" \
		-e "s|^Exec=.*ghostty --gtk-single-instance=true|Exec=${ghostty_bin} --gtk-single-instance=true|" \
		-e "s|^DBusActivatable=.*|DBusActivatable=false|" \
		"$path" | write_stdin_if_changed "$path" 0644
}

rewrite_ghostty_service_file() {
	local path=${1:?service file is required}

	sed \
		-e "s|^Exec=/work/stage/bin/ghostty|Exec=${ghostty_bin}|" \
		-e "s|^ExecStart=/work/stage/bin/ghostty|ExecStart=${ghostty_bin}|" \
		"$path" | write_stdin_if_changed "$path" 0644
}

write_check_state() {
	local source_key=${1:?source key is required}
	local now

	now=$(now_seconds)
	printf '%s\n' "$source_key" | write_stdin_if_changed "$source_key_file" 0644
	printf '%s\n' "$now" | write_stdin_if_changed "$checked_at_file" 0644
	printf '%s\n' "$state_version" | write_stdin_if_changed "$state_version_file" 0644
}

require_command cp date tar

ensure_dirs "$cache_dir" "$install_prefix"

if fresh_check_exists; then
	printf 'Ghostty tip was checked less than %s seconds ago; skipping.\n' "$check_interval_seconds"
	exit 0
fi

if [[ -x $ghostty_bin ]]; then
	verify_runtime_libraries
fi

require_command gh podman

if ! gh auth status >/dev/null 2>&1; then
	printf '%s\n' 'error: gh must be authenticated to download the Ghostty tip tarball.' >&2
	printf '%s\n' 'Run gh auth login and retry.' >&2
	exit 1
fi

build_log="${cache_dir%/}/ghostty-tip-build.log"
work_dir=$(mktemp -d "${cache_dir%/}/build.XXXXXXXXXX")
trap cleanup EXIT

download_dir="${work_dir}/download"
source_dir="${work_dir}/source"
stage_dir="${work_dir}/stage"
ensure_dirs "$download_dir" "$source_dir" "$stage_dir"

source_key=$(
	gh release view tip \
		--repo "$repo" \
		--json assets \
		--jq '.assets[] | select(.name == "ghostty-source.tar.gz") | [.name, .size, .updatedAt] | @tsv'
)
if [[ -z $source_key ]]; then
	printf '%s\n' 'error: Ghostty tip release does not include ghostty-source.tar.gz.' >&2
	exit 1
fi

if [[ -x $ghostty_bin && -r $source_key_file && $(<"$source_key_file") == "$source_key" ]]; then
	verify_runtime_libraries
	write_check_state "$source_key"
	printf 'Ghostty tip source is already current.\n'
	exit 0
fi

gh release download tip \
	--repo "$repo" \
	--pattern ghostty-source.tar.gz \
	--clobber \
	--dir "$download_dir"

tar -xf "${download_dir}/ghostty-source.tar.gz" -C "$source_dir"
extracted_source=$(find "$source_dir" -mindepth 1 -maxdepth 1 -type d -print -quit)
if [[ -z $extracted_source ]]; then
	printf '%s\n' 'error: Ghostty source tarball did not contain a source directory.' >&2
	exit 1
fi

mv "$extracted_source" "${work_dir}/ghostty"

if ! podman run --rm \
	--security-opt label=disable \
	--volume "${work_dir}:/work" \
	--workdir /work/ghostty \
	--env "ZIG_ARCH=$(zig_arch)" \
	--env "ZIG_VERSION=${zig_version}" \
	"$container_image" \
	bash -ceu '
    dnf -y install --setopt=install_weak_deps=False \
      ca-certificates \
      curl \
      file \
      findutils \
      gcc \
      gcc-c++ \
      gettext \
      glibc-devel \
      gtk4-devel \
      gtk4-layer-shell-devel \
      libadwaita-devel \
      libxml2 \
      pkgconf-pkg-config \
      tar \
      xz

    zig_name="zig-${ZIG_ARCH}-${ZIG_VERSION}"
    curl -fsSLO "https://ziglang.org/download/${ZIG_VERSION}/${zig_name}.tar.xz"
    tar -xf "${zig_name}.tar.xz" -C /opt
    export PATH="/opt/${zig_name}:${PATH}"

    zig build -p /work/stage -Doptimize=ReleaseFast
  ' >"$build_log" 2>&1; then
	printf 'error: Ghostty build failed; tail of %s follows.\n' "$build_log" >&2
	tail -n 160 "$build_log" >&2 || true
	exit 1
fi

built_ghostty_bin="${stage_dir}/bin/ghostty"
pending_ghostty_bin=
if [[ -f $built_ghostty_bin ]]; then
	pending_ghostty_bin="${work_dir}/.ghostty-bin"
	mv "$built_ghostty_bin" "$pending_ghostty_bin"
fi

cp -R --reflink=auto --no-preserve=ownership,timestamps "${stage_dir}/." "$install_prefix/"

if [[ -n $pending_ghostty_bin ]]; then
	ensure_dirs "${install_prefix%/}/bin"
	ghostty_bin_tmp=$(mktemp "${install_prefix%/}/bin/.ghostty.XXXXXXXXXX")
	cp --reflink=auto --no-preserve=ownership,timestamps "$pending_ghostty_bin" "$ghostty_bin_tmp"
	chmod --reference="$pending_ghostty_bin" "$ghostty_bin_tmp"
	mv -f "$ghostty_bin_tmp" "$ghostty_bin"
fi

desktop_file="${install_prefix}/share/applications/com.mitchellh.ghostty.desktop"
if [[ -f $desktop_file ]]; then
	rewrite_ghostty_desktop_file "$desktop_file"
fi

for service_file in \
	"${install_prefix}/share/dbus-1/services/com.mitchellh.ghostty.service" \
	"${install_prefix}/share/systemd/user/app-com.mitchellh.ghostty.service"; do
	if [[ -f $service_file ]]; then
		rewrite_ghostty_service_file "$service_file"
	fi
done

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "${install_prefix}/share/applications" >/dev/null 2>&1 || true
fi

verify_runtime_libraries
write_check_state "$source_key"
printf 'Installed native Ghostty tip release build into %s.\n' "$install_prefix"
