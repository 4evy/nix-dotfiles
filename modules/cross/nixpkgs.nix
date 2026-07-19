{ inputs, ... }:
{
  _class = null;

  nixpkgs = {
    config.allowUnfree = true;
    overlays = [
      (
        _: prev:
        let
          unstable = import inputs.nixpkgs-unstable {
            inherit (prev.stdenvNoCC.hostPlatform) system;
            inherit (prev) config;
          };
          eupkgsScope =
            unstable
            // eupkgsOverlay
            // {
              callPackage = unstable.lib.callPackageWith eupkgsScope;
            };
          eupkgsOverlay = inputs.eupkgs.overlays.default eupkgsScope unstable;
          eupkgs = removeAttrs eupkgsOverlay [ "_internalCallByNamePackageFile" ];
        in
        {
          inherit unstable eupkgs;
        }
      )
      (
        final: prev:
        let
          goWorkspacePackage = final.callPackage ../../packages/go-workspace-package.nix { };
          spectrumSourcePins = builtins.fromJSON (
            builtins.readFile ../../spectrum/scripts/spectrum_build/programs/source-pins.json
          );
        in
        {
          inherit spectrumSourcePins;
          gh = final.unstable.gh;
          lldb-mcp-launcher = final.eupkgs.lldb-mcp-launcher;
          ghidra-mcp-headless = final.eupkgs.ghidra-mcp-headless;
          ghidra-mcp = final.callPackage ../../packages/ghidra-mcp.nix {
            inherit (final) ghidra-mcp-headless;
          };
          kanata = prev.kanata;
          kanata-with-cmd = final.kanata.override { withCmd = true; };
          kmscon = prev.kmscon.overrideAttrs (
            _: previousAttrs: {
              inherit (spectrumSourcePins.kmscon) version;
              src = final.fetchFromGitHub {
                owner = "kmscon";
                repo = "kmscon";
                rev = spectrumSourcePins.kmscon.revision;
                hash = spectrumSourcePins.kmscon.source_sha256;
              };
              buildInputs = previousAttrs.buildInputs ++ [ final.dbus ];
              # 10.0.1 installs kmscon itself as an ELF binary; nixpkgs'
              # 10.0.0 fixup still tries to rewrite it as a shell script.
              postFixup = ''
                substituteInPlace $out/bin/kmscon-launch-gui \
                  --replace-fail "inotifywait" "${final.lib.getExe' final.inotify-tools "inotifywait"}"
              '';
            }
          );
          hyper-window-tiling = final.callPackage ../../packages/hyper-window-tiling.nix { };
          hyper-window-tiling-gnome = final.hyper-window-tiling.gnome;
          hyper-window-tiling-kde = final.hyper-window-tiling.kde;
          dotfiles-python = final.callPackage ../../packages/dotfiles-python.nix {
            inherit (final.unstable) python314Packages;
          };
          # Keep the privileged command on the same packaged entry point as
          # every other repository automation command. Copying its module into
          # a standalone script loses the Python dependency environment.
          system-runner = final.dotfiles-python;
          uresourced = final.callPackage ../../packages/uresourced.nix {
            sourcePin = spectrumSourcePins.uresourced;
          };
          terminal-theme-tools = goWorkspacePackage {
            pname = "terminal-theme-tools";
            subPackages = [ "cmd/terminal-theme-run" ];
            meta = {
              description = "Theme-aware wrappers for terminal applications";
              mainProgram = "terminal-theme-run";
              platforms = final.lib.platforms.linux;
            };
          };
        }
      )
    ];
  };
}
