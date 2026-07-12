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
  heliumPrivateSettingsFile = config.sops.secrets."helium-cookie-autodelete-settings".path;

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
    "--load-extension=${cookieAutoDeleteExtension}/share/helium/extensions/${cookieAutoDeleteId}"
  ];

  chromeStoreUpdateUrl = "https://clients2.google.com/service/update2/crx";

  chromeStoreExtensionIds = [
    # 1Password - Password Manager
    "aeblfdkhhhdcdjpifhhbdiojplfjncoa"
    # Catppuccin for Web File Explorer Icons
    "lnjaiaapbakfhlbjenjkhffcdpoompki"
    # Enhancer for YouTube
    "ponfpcnoihfmfllpaingbgckeeldkhle"
    # Minimal Theme for Twitter
    "pobhoodpcipjmedfenaigbeloiidbflp"
    # All-in-one bookmark manager
    "ldgfbffkinooeloadekpmfoklnobpien"
    # SponsorBlock for YouTube - Skip Sponsorships
    "mnjggcdmjocbbbhaepdhchncahnbgone"
    # Refined GitHub
    "hlepfoohegkhhmjieoechaddaejaokhf"
  ];

  twpExtension = {
    id = "bolggfoncklhniejomgplkjcllmnonbh";
    version = "10.1.1.0";
    crxPath = pkgs.fetchurl {
      url = "https://github.com/FilipePS/Traduzir-paginas-web/releases/download/v10.1.1.0/TWP_10.1.1.0_Chromium.crx";
      name = "bolggfoncklhniejomgplkjcllmnonbh.crx";
      hash = "sha256-X4m1To1n/1zQGrzQPXPyR8KIA4JleyyAh5AjuS2BvYw=";
    };
  };

  cookieAutoDeleteId = "hebmefdjnehapihcomeennjpdjghcpdn";

  cookieAutoDeleteExtension = pkgs.stdenvNoCC.mkDerivation {
    pname = "helium-cookie-autodelete-extension";
    version = "3.8.2";

    src = pkgs.fetchurl {
      url = "https://github.com/Cookie-AutoDelete/Cookie-AutoDelete/releases/download/v3.8.2/Cookie-AutoDelete_v3.8.2_Chrome.zip";
      hash = "sha256-dzSNl4/W42sd8J+lJGOoFa/Znh6cdoxg8jZSkM6+dyM=";
    };

    nativeBuildInputs = [ pkgs.unzip ];

    unpackPhase = ''
      runHook preUnpack
      unzip -q "$src" -d source
      runHook postUnpack
    '';

    installPhase = ''
      runHook preInstall
      extension_dir="$out/share/helium/extensions/${cookieAutoDeleteId}"
      mkdir -p "$extension_dir"
      cp -R source/. "$extension_dir/"
      runHook postInstall
    '';
  };

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
    id:
    externalExtensionFile id {
      external_update_url = chromeStoreUpdateUrl;
    };

  twpExternalExtensionFile = externalExtensionFile twpExtension.id {
    external_crx = twpExtension.crxPath;
    external_version = twpExtension.version;
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
      [ twpExternalExtensionFile ] ++ map chromeStoreExternalExtensionFile chromeStoreExtensionIds
    );

    sops.secrets."helium-cookie-autodelete-settings" = {
      owner = "4evy";
      mode = "0400";
    };

    system.activationScripts.heliumExtensionSettings = {
      deps = [
        "setupSecrets"
        "users"
      ];
      text = ''
        mkdir -p '${heliumProfileDir}'
        chown 4evy:users '/home/4evy/.config' '/home/4evy/.config/net.imput.helium' '${heliumProfileDir}' 2>/dev/null || true

        if command -v runuser >/dev/null 2>&1; then
          runuser -u 4evy -- ${heliumBrowserTool}/bin/helium-browser apply-extension-settings \
            --profile-dir '${heliumProfileDir}' \
            --settings '${heliumPrivateSettingsFile}' \
            --gh-token || true
        else
          su -s /bin/sh 4evy -c '${heliumBrowserTool}/bin/helium-browser apply-extension-settings --profile-dir ${heliumProfileDir} --settings ${heliumPrivateSettingsFile} --gh-token' || true
        fi
      '';
    };
  };
}
