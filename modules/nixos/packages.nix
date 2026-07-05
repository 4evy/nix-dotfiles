{
  config,
  lib,
  pkgs,
  ...
}:
let
  inherit (lib.attrsets) attrValues;
  inherit (lib.modules) mkIf mkMerge;
  inherit (lib.options) mkEnableOption;
  inherit (lib.strings) makeSearchPath;
  rustPkgConfigPath = makeSearchPath "lib/pkgconfig" [
    pkgs.unstable.openssl.dev
    pkgs.unstable.zlib.dev
  ];
in
{
  _class = "nixos";

  options.local.toolbox.enable = mkEnableOption "Distrobox prerequisites for Linux toolbox tools that are awkward outside Nix";

  config = mkMerge [
    {
      environment.systemPackages = attrValues {
        inherit (pkgs)
          ansible
          ansible-lint
          system-runner
          yamllint
          zellij-theme-tools
          ;

        # Host/session spine and editor dependencies.
        inherit (pkgs.unstable)
          actionlint
          bash-language-server
          binutils
          cargo
          clang
          clippy
          cmake
          gcc
          gh
          git
          git-lfs
          go
          golangci-lint
          gopls
          gnumake
          helix
          lld
          lldb
          nil
          nixd
          nixfmt
          openssl
          openssh_hpn
          patch
          perl
          pkg-config
          rust-bindgen
          rust-analyzer
          rustc
          rustfmt
          shellcheck
          shfmt
          taplo
          vscode
          yaml-language-server
          zellij
          ;

        # Hardware and platform tools.
        inherit (pkgs.unstable)
          chezmoi
          ghostty
          nh
          pciutils
          smartmontools
          wl-clipboard
          ;
        inherit (pkgs) dotool;
      };

      environment.sessionVariables = {
        LIBCLANG_PATH = "${pkgs.unstable.llvmPackages.libclang.lib}/lib";
        PKG_CONFIG_PATH = rustPkgConfigPath;
        RUST_SRC_PATH = "${pkgs.unstable.rustPlatform.rustLibSrc}";
      };
    }
    (mkIf config.local.toolbox.enable {
      environment.systemPackages = attrValues {
        inherit (pkgs) distrobox podman-compose;
      };
      virtualisation.podman = {
        enable = true;
        dockerCompat = true;
        defaultNetwork.settings.dns_enabled = true;
      };
    })
  ];
}
