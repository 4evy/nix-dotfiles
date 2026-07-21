{
  config,
  lib,
  pkgs,
  ...
}:
let
  inherit (lib.attrsets) attrValues;
  inherit (lib.modules) mkIf;
  inherit (lib.strings) concatStringsSep;

  desktopEnabled = config.local.gnome.enable || config.local.kde.enable;
  heliumProfileDir = "/home/4evy/.config/net.imput.helium/Default";

  chromiumFeatures = [
    "ForceEnableWebGpuInterop"
    "ReduceOpsTaskSplitting"
    "TouchpadOverscrollHistoryNavigation"
    "VaapiVideoDecoder"
    "VaapiVideoEncoder"
    "BrowsingTopics"
    "InterestGroupStorage"
  ];

  chromiumDisabledFeatures = [
    "ExtensionManifestV2Unsupported"
    "ExtensionManifestV2Disabled"
  ];

  commandLineArgs = [
    "--enable-logging=stderr"
    "--enable-features=${concatStringsSep "," chromiumFeatures}"
    "--disable-features=${concatStringsSep "," chromiumDisabledFeatures}"
    "--omnibox-autocomplete-filtering=search"
    "--set-user-color=244,184,228"
    "--set-color-scheme=dark"
    "--set-color-variant=tonal_spot"
    "--ignore-gpu-blocklist"
    "--enable-wayland-ime"
    "--wayland-text-input-version=3"
  ];

  extensionCatalog = builtins.fromTOML (
    builtins.readFile ../../internal/chromiumbrowser/extensions/extensions.toml
  );

  heliumBrowserTool = pkgs.callPackage ../../packages/go-workspace-package.nix { } {
    pname = "helium-browser";
    subPackages = [ "cmd/helium-browser" ];

    meta = {
      description = "Install and configure Helium browser";
      mainProgram = "helium-browser";
    };
  };

  externalExtensionFile = id: value: {
    name = "xdg/net.imput.helium/External Extensions/${id}.json";
    value.text = builtins.toJSON value;
  };

  chromeStoreExternalExtensionFile =
    extension:
    externalExtensionFile extension.id {
      external_update_url = extensionCatalog.chrome_store_update_url;
    };

  updateUrlExternalExtensionFile =
    extension:
    externalExtensionFile extension.id {
      external_update_url = extension.update_url;
    };

  crxExternalExtensionFile =
    extension:
    externalExtensionFile extension.id {
      external_crx = pkgs.fetchurl {
        inherit (extension) url;
        name = "${extension.id}.crx";
        hash = extension.sha256;
      };
      external_version = extension.version;
    };

  heliumBrowser = pkgs.eupkgs.helium-browser.override {
    commandLineArgs = concatStringsSep " " commandLineArgs;
  };
in
{
  config = mkIf desktopEnabled {
    programs.chromium.enable = true;

    environment.systemPackages = attrValues {
      inherit heliumBrowser;

      inherit (pkgs.unstable)
        networkmanagerapplet
        nufraw-thumbnailer
        pavucontrol
        playerctl
        telegram-desktop
        ;
    };

    environment.etc = builtins.listToAttrs (
      map chromeStoreExternalExtensionFile extensionCatalog.chrome_store_extensions
      ++ map updateUrlExternalExtensionFile extensionCatalog.update_url_extensions
      ++ map crxExternalExtensionFile extensionCatalog.crx_extensions
    );

    system.activationScripts.heliumProfileSettings = {
      deps = [ "users" ];
      text = ''
        mkdir -p '${heliumProfileDir}'
        chown 4evy:users '/home/4evy/.config' '/home/4evy/.config/net.imput.helium' '${heliumProfileDir}' 2>/dev/null || true

        if command -v runuser >/dev/null 2>&1; then
          token="$(runuser -u 4evy -- ${pkgs.gh}/bin/gh auth token 2>/dev/null || true)"
          input="$(${pkgs.jq}/bin/jq -nc --arg token "$token" \
            '{extension_values: (if $token == "" then {} else {"refined-github-personal-token": $token} end)}')"
          printf '%s' "$input" | runuser -u 4evy -- ${heliumBrowserTool}/bin/helium-browser apply-profile-settings \
            --profile-dir '${heliumProfileDir}' \
            --input - || true
        else
          token="$(su -s /bin/sh 4evy -c '${pkgs.gh}/bin/gh auth token' 2>/dev/null || true)"
          input="$(${pkgs.jq}/bin/jq -nc --arg token "$token" \
            '{extension_values: (if $token == "" then {} else {"refined-github-personal-token": $token} end)}')"
          printf '%s' "$input" | su -s /bin/sh 4evy -c \
            '${heliumBrowserTool}/bin/helium-browser apply-profile-settings --profile-dir ${heliumProfileDir} --input -' || true
        fi
      '';
    };
  };
}
