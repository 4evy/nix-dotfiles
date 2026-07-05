#!/usr/bin/env bash
# shellcheck shell=bash

helium_arch() {
	case "$(uname -m)" in
		arm64 | aarch64)
			printf '%s\n' arm64
			;;
		x86_64 | amd64)
			printf '%s\n' x86_64
			;;
		*)
			die "unsupported Helium architecture: $(uname -m)"
			;;
	esac
}

helium_latest_release() {
	local repository=${1:?repository is required}

	curl_stdout "https://api.github.com/repos/$repository/releases/latest"
}

helium_release_tag() {
	local release_json=${1:?release JSON is required}

	jq_read_text '.tag_name' "$release_json"
}

helium_release_asset_field() {
	local release_json=${1:?release JSON is required}
	local asset_name=${2:?asset name is required}
	local field=${3:?asset field is required}

	require_command jq
	jq -er \
		--arg name "$asset_name" \
		--arg field "$field" \
		'.assets[] | select(.name == $name) | .[$field] // empty' \
		<<<"$release_json"
}

helium_asset_download_url() {
	helium_release_asset_field "$1" "$2" browser_download_url
}

helium_asset_digest() {
	helium_release_asset_field "$1" "$2" digest 2>/dev/null || true
}

helium_sha256_check() {
	local expected=${1:?expected sha256 is required}
	local path=${2:?file path is required}

	if command -v sha256sum >/dev/null 2>&1; then
		printf '%s  %s\n' "$expected" "$path" | sha256sum -c - >/dev/null
		return
	fi

	require_command shasum
	printf '%s  %s\n' "$expected" "$path" | shasum -a 256 -c - >/dev/null
}

helium_verify_asset() {
	local path=${1:?asset path is required}
	local digest=${2-}
	local expected

	if [[ -z $digest ]]; then
		return 0
	fi
	if [[ $digest != sha256:* ]]; then
		return 0
	fi
	expected=${digest#sha256:}
	if [[ ! $expected =~ ^[0-9a-fA-F]{64}$ ]]; then
		die "invalid Helium sha256 digest: $digest"
	fi
	helium_sha256_check "${expected,,}" "$path"
}

helium_ensure_downloaded() {
	local path=${1:?asset path is required}
	local url=${2:?asset URL is required}
	local digest=${3-}
	local temp

	if [[ -f $path ]] && helium_verify_asset "$path" "$digest"; then
		printf 'helium-browser: using cached %s\n' "${path##*/}" >&2
		return 0
	fi

	if [[ -f $path ]]; then
		printf 'helium-browser: cached %s failed verification; downloading again\n' "${path##*/}" >&2
	else
		printf 'helium-browser: downloading %s\n' "${path##*/}" >&2
	fi

	require_command mktemp rm
	ensure_dir "${path%/*}"
	temp=$(mktemp --tmpdir="${path%/*}" ".${path##*/}.download.XXXXXX")
	if ! curl_download "$url" "$temp"; then
		rm -f -- "$temp"
		return 1
	fi
	if ! helium_verify_asset "$temp" "$digest"; then
		rm -f -- "$temp"
		return 1
	fi
	if ! install_file_if_changed "$temp" "$path" 0644; then
		rm -f -- "$temp"
		return 1
	fi
	rm -f -- "$temp"
}
