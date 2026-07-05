#!/usr/bin/env bash
# shellcheck shell=bash
## usage: install-ghidra-mcp [--version] [--force] [--ghidra-home <path>] [<cache-dir> [<install-prefix> [<bin-dir>]]]
##
## Builds the pinned bethington/ghidra-mcp headless server and Python bridge
## outside Nix, using the same inputs and launcher behavior as eupkgs.
##
## Defaults:
##   cache-dir       ${XDG_CACHE_HOME:-$HOME/.cache}/dotfiles/ghidra-mcp
##   install-prefix  $HOME/.local/opt/ghidra-mcp/latest
##   bin-dir         $HOME/.local/bin
set -euo pipefail

script_dir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
package_version=5.15.0-unstable-2026-07-03
jar_version=5.15.0
upstream_rev=b2d16d7dadb114094a2ca13fbf961e79b83f55b4
upstream_url=https://github.com/bethington/ghidra-mcp.git
mcp_sdk_version=1.28.1
reproducible_build_stamp=19700101-000000
state_version=1

usage() {
	sed -n 's/^##[[:space:]]\{0,1\}//p' "${BASH_SOURCE[0]}"
}

die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

require_command() {
	local command

	for command; do
		command -v "$command" >/dev/null 2>&1 || die "missing required command: $command"
	done
}

ensure_dir() {
	local dir=${1:?directory is required}

	mkdir -p -- "$dir"
}

safe_to_replace() {
	local path=${1:?path is required}

	[[ -n $path ]] || return 1
	[[ $path != / ]] || return 1
	[[ $path != . ]] || return 1
	[[ $path != .. ]] || return 1
	[[ $path != "$HOME" ]] || return 1
}

remove_path() {
	local path=${1:?path is required}

	safe_to_replace "$path" || die "refusing to remove unsafe path: $path"
	rm -rf -- "$path"
}

shell_quote() {
	printf "'"
	printf '%s' "$1" | sed "s/'/'\\\\''/g"
	printf "'"
}

write_executable() {
	local path=${1:?path is required}

	ensure_dir "${path%/*}"
	cat >"$path"
	chmod 0755 "$path"
}

data_home() {
	if [[ -n ${XDG_DATA_HOME:-} ]]; then
		printf '%s\n' "$XDG_DATA_HOME"
	else
		printf '%s\n' "$HOME/.local/share"
	fi
}

detect_support_dir() {
	local candidate

	for candidate in \
		"${GHIDRA_MCP_INSTALLER_SUPPORT_DIR:-}" \
		"$script_dir/ghidra-mcp" \
		"$(data_home)/dotfiles/scripts/ghidra-mcp"; do
		[[ -n $candidate ]] || continue
		if [[ -f $candidate/bridge-auth-token.patch && -f $candidate/wrappers/ghidra-mcp-serve.sh.in ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done

	return 1
}

default_cache_dir() {
	if [[ -n ${XDG_CACHE_HOME:-} ]]; then
		printf '%s\n' "$XDG_CACHE_HOME/dotfiles/ghidra-mcp"
	else
		printf '%s\n' "$HOME/.cache/dotfiles/ghidra-mcp"
	fi
}

detect_ghidra_home() {
	local candidate
	local ghidra_dir
	local brew_prefix

	for candidate in "${GHIDRA_HOME:-}" "${GHIDRA_INSTALL_DIR:-}" "${GHIDRA_ROOT:-}"; do
		[[ -n $candidate ]] || continue
		if [[ -d $candidate/Ghidra/Features && -d $candidate/Ghidra/Framework ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
		if [[ -d $candidate/Features && -d $candidate/Framework ]]; then
			printf '%s\n' "${candidate%/Ghidra}"
			return 0
		fi
	done

	if command -v brew >/dev/null 2>&1 && brew_prefix=$(brew --prefix ghidra 2>/dev/null); then
		for candidate in \
			"$brew_prefix/libexec" \
			"$brew_prefix" \
			"$brew_prefix/share/ghidra" \
			"$brew_prefix/ghidra"; do
			if [[ -d $candidate/Ghidra/Features && -d $candidate/Ghidra/Framework ]]; then
				printf '%s\n' "$candidate"
				return 0
			fi
		done
	fi

	while IFS= read -r ghidra_dir; do
		candidate=${ghidra_dir%/Ghidra}
		if [[ -d $candidate/Ghidra/Features && -d $candidate/Ghidra/Framework ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done < <(
		for candidate in \
			/opt/homebrew \
			/usr/local \
			/home/linuxbrew/.linuxbrew \
			/Applications \
			"$HOME/Applications" \
			"$HOME/.local/share" \
			/opt \
			/usr/share; do
			[[ -d $candidate ]] || continue
			find "$candidate" -maxdepth 7 -type d -name Ghidra -print 2>/dev/null
		done
	)

	return 1
}

ghidra_version() {
	local ghidra_home=${1:?Ghidra home is required}
	local properties
	local version

	properties=$(find "$ghidra_home" -maxdepth 4 -type f -name application.properties -print -quit 2>/dev/null || true)
	if [[ -n $properties ]]; then
		version=$(sed -n -E 's/^[[:space:]]*application\.version[[:space:]]*=[[:space:]]*([^[:space:]]+).*/\1/p' "$properties" | head -n 1)
		if [[ -n $version ]]; then
			printf '%s\n' "$version"
			return 0
		fi
	fi

	if [[ $ghidra_home =~ ghidra[_-]([0-9][0-9A-Za-z._-]*) ]]; then
		printf '%s\n' "${BASH_REMATCH[1]%%_PUBLIC*}"
		return 0
	fi

	return 1
}

required_ghidra_jars() {
	cat "$support_dir/required-ghidra-jars.txt"
}

verify_ghidra_jars() {
	local ghidra_home=${1:?Ghidra home is required}
	local path
	local missing=0

	while IFS= read -r path; do
		[[ -f $ghidra_home/Ghidra/$path ]] && continue
		printf 'missing Ghidra jar: %s\n' "$ghidra_home/Ghidra/$path" >&2
		missing=1
	done < <(required_ghidra_jars)

	((missing == 0)) || die "Ghidra installation is missing jars required by ghidra-mcp"
}

java_major_version() {
	local version

	version=$(java -version 2>&1 | sed -n -E 's/.* version "([^"]+)".*/\1/p; s/^openjdk ([^ ]+).*/\1/p' | head -n 1)
	[[ -n $version ]] || return 1
	if [[ $version == 1.* ]]; then
		printf '%s\n' "${version#1.}" | cut -d. -f1
	else
		printf '%s\n' "$version" | cut -d. -f1
	fi
}

install_ghidra_maven_deps() {
	local ghidra_home=${1:?Ghidra home is required}
	local version=${2:?Ghidra version is required}
	local m2_repo=${3:?Maven repo is required}
	local jar_path
	local artifact

	ensure_dir "$m2_repo"
	while IFS= read -r jar_path; do
		artifact=${jar_path##*/}
		artifact=${artifact%.jar}
		mvn -q org.apache.maven.plugins:maven-install-plugin:3.1.2:install-file \
			-Dmaven.repo.local="$m2_repo" \
			-Dfile="$ghidra_home/Ghidra/$jar_path" \
			-DgroupId=ghidra \
			-DartifactId="$artifact" \
			-Dversion="$version" \
			-Dpackaging=jar \
			-DgeneratePom=true
	done < <(required_ghidra_jars)
}

checkout_source() {
	local source_dir=${1:?source dir is required}

	if [[ -d $source_dir/.git ]]; then
		git -C "$source_dir" fetch --depth 1 origin "$upstream_rev"
	else
		remove_path "$source_dir"
		git clone --no-checkout "$upstream_url" "$source_dir"
		git -C "$source_dir" fetch --depth 1 origin "$upstream_rev"
	fi
	git -C "$source_dir" checkout --force "$upstream_rev"
	git -C "$source_dir" clean -fdx
	[[ $(git -C "$source_dir" rev-parse HEAD) == "$upstream_rev" ]] || die "failed to checkout $upstream_rev"
}

patch_source() {
	local source_dir=${1:?source dir is required}

	patch -d "$source_dir" -p1 <"$support_dir/bridge-auth-token.patch"
}

rewrite_pom() {
	local source_dir=${1:?source dir is required}
	local version=${2:?Ghidra version is required}

	python3 "$support_dir/rewrite-pom.py" "$source_dir/pom.xml" "$version" "$reproducible_build_stamp"
}

build_server() {
	local source_dir=${1:?source dir is required}
	local ghidra_home=${2:?Ghidra home is required}
	local version=${3:?Ghidra version is required}
	local work_dir=${4:?work dir is required}
	local stage_dir=${5:?stage dir is required}
	local m2_repo="$work_dir/m2"

	install_ghidra_maven_deps "$ghidra_home" "$version" "$m2_repo"
	rewrite_pom "$source_dir" "$version"

	(
		cd "$source_dir"
		mvn -Pheadless \
			-Dmaven.repo.local="$m2_repo" \
			-DskipTests \
			-Djacoco.skip=true \
			package
	)

	[[ -f $source_dir/target/GhidraMCP-${jar_version}.jar ]] || die "Maven build did not produce target/GhidraMCP-${jar_version}.jar"
	ensure_dir "$stage_dir/share/java"
	cp "$source_dir/target/GhidraMCP-${jar_version}.jar" "$stage_dir/share/java/GhidraMCP-${jar_version}.jar"
}

build_bridge() {
	local source_dir=${1:?source dir is required}
	local stage_dir=${2:?stage dir is required}
	local venv_dir="$stage_dir/venv"

	python3 -m venv "$venv_dir"
	"$venv_dir/bin/python" -m pip install --upgrade pip wheel hatchling
	"$venv_dir/bin/python" -m pip install "mcp==${mcp_sdk_version}"
	"$venv_dir/bin/python" -m pip install --no-deps "$source_dir"
}

sed_escape() {
	printf '%s' "$1" | sed -e 's/[\/&|\\]/\\&/g'
}

render_wrapper() {
	local template=${1:?template is required}
	local path=${2:?path is required}
	local install_root=${3:?install root is required}
	local ghidra_home=${4:-}

	sed \
		-e "s|@INSTALL_ROOT@|$(sed_escape "$(shell_quote "$install_root")")|g" \
		-e "s|@GHIDRA_HOME@|$(sed_escape "$(shell_quote "$ghidra_home")")|g" \
		-e "s|@JAR_VERSION@|$(sed_escape "$jar_version")|g" \
		"$template" | write_executable "$path"
}

link_bins() {
	local install_prefix=${1:?install prefix is required}
	local bin_dir=${2:?bin dir is required}
	local bin

	ensure_dir "$bin_dir"
	for bin in ghidra-mcp-httpd ghidra-mcp-bridge ghidra-mcp-headless ghidra-mcp-serve; do
		ln -sfn "$install_prefix/bin/$bin" "$bin_dir/$bin"
	done
}

force=0
ghidra_home_arg=
while (($# > 0)); do
	case $1 in
		--version)
			printf 'install-ghidra-mcp %s\n' "$package_version"
			exit 0
			;;
		-h | --help)
			usage
			exit 0
			;;
		--force)
			force=1
			shift
			;;
		--ghidra-home)
			(($# >= 2)) || die "--ghidra-home requires a path"
			ghidra_home_arg=$2
			shift 2
			;;
		--)
			shift
			break
			;;
		-*)
			die "unknown option: $1"
			;;
		*)
			break
			;;
	esac
done

(($# <= 3)) || {
	usage >&2
	exit 2
}

cache_dir=${1:-$(default_cache_dir)}
install_prefix=${2:-$HOME/.local/opt/ghidra-mcp/latest}
bin_dir=${3:-$HOME/.local/bin}
source_dir="${cache_dir%/}/source"
build_log="${cache_dir%/}/ghidra-mcp-build.log"
stamp_file="${install_prefix%/}/.ghidra-mcp-build"

require_command bash chmod cp curl date find git java mvn patch python3 rm sed

if ! support_dir=$(detect_support_dir); then
	die "could not find ghidra-mcp installer support files; set GHIDRA_MCP_INSTALLER_SUPPORT_DIR"
fi

java_major=$(java_major_version || true)
[[ -n $java_major ]] || die "could not determine Java version"
((java_major >= 21)) || die "Java 21 or newer is required; found Java $java_major"

if [[ -n $ghidra_home_arg ]]; then
	ghidra_home=$ghidra_home_arg
elif ! ghidra_home=$(detect_ghidra_home); then
	die "could not find Ghidra; pass --ghidra-home PATH where PATH contains the Ghidra/ directory"
fi
[[ -d $ghidra_home/Ghidra ]] || die "Ghidra home must contain a Ghidra/ directory: $ghidra_home"
verify_ghidra_jars "$ghidra_home"

if ! detected_ghidra_version=$(ghidra_version "$ghidra_home"); then
	die "could not determine Ghidra version from $ghidra_home"
fi

build_key=$(
	printf 'state_version=%s\n' "$state_version"
	printf 'package_version=%s\n' "$package_version"
	printf 'upstream_rev=%s\n' "$upstream_rev"
	printf 'jar_version=%s\n' "$jar_version"
	printf 'mcp_sdk_version=%s\n' "$mcp_sdk_version"
	printf 'ghidra_home=%s\n' "$ghidra_home"
	printf 'ghidra_version=%s\n' "$detected_ghidra_version"
)

if ((force == 0)) &&
	[[ -x $install_prefix/bin/ghidra-mcp-serve ]] &&
	[[ -x $install_prefix/bin/ghidra-mcp-httpd ]] &&
	[[ -x $install_prefix/bin/ghidra-mcp-bridge ]] &&
	[[ -r $stamp_file ]] &&
	[[ $(<"$stamp_file") == "$build_key" ]]; then
	link_bins "$install_prefix" "$bin_dir"
	printf 'Ghidra MCP already current at %s for Ghidra %s.\n' "$package_version" "$detected_ghidra_version"
	exit 0
fi

ensure_dir "$cache_dir"
checkout_source "$source_dir"
patch_source "$source_dir"

work_dir=$(mktemp -d "${cache_dir%/}/build.XXXXXXXXXX")
stage_dir="$work_dir/stage"
cleanup() {
	remove_path "$work_dir"
}
trap cleanup EXIT
ensure_dir "$stage_dir/bin"

printf 'Building Ghidra MCP %s against Ghidra %s at %s.\n' "$package_version" "$detected_ghidra_version" "$ghidra_home"
if ! (
	set -euo pipefail
	build_server "$source_dir" "$ghidra_home" "$detected_ghidra_version" "$work_dir" "$stage_dir"
	build_bridge "$source_dir" "$stage_dir"
	render_wrapper "$support_dir/wrappers/ghidra-mcp-httpd.sh.in" "$stage_dir/bin/ghidra-mcp-httpd" "$install_prefix" "$ghidra_home"
	render_wrapper "$support_dir/wrappers/ghidra-mcp-bridge.sh.in" "$stage_dir/bin/ghidra-mcp-bridge" "$install_prefix"
	render_wrapper "$support_dir/wrappers/ghidra-mcp-headless.sh.in" "$stage_dir/bin/ghidra-mcp-headless" "$install_prefix"
	render_wrapper "$support_dir/wrappers/ghidra-mcp-serve.sh.in" "$stage_dir/bin/ghidra-mcp-serve" "$install_prefix"
	printf '%s\n' "$build_key" >"$stage_dir/.ghidra-mcp-build"
) >"$build_log" 2>&1; then
	printf 'error: Ghidra MCP build failed; tail of %s follows.\n' "$build_log" >&2
	tail -n 160 "$build_log" >&2 || true
	exit 1
fi

safe_to_replace "$install_prefix" || die "refusing to replace unsafe install prefix: $install_prefix"
ensure_dir "${install_prefix%/*}"
remove_path "$install_prefix"
mv "$stage_dir" "$install_prefix"
trap - EXIT
remove_path "$work_dir"

link_bins "$install_prefix" "$bin_dir"
printf 'Installed Ghidra MCP %s into %s and linked launchers in %s.\n' "$package_version" "$install_prefix" "$bin_dir"
