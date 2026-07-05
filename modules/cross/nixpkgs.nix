{ inputs, ... }:
{
  _class = null;

  nixpkgs = {
    config.allowUnfree = true;
    overlays = [
      (
        final: prev:
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
          eupkgs = builtins.removeAttrs eupkgsOverlay [ "_internalCallByNamePackageFile" ];
        in
        {
          inherit unstable eupkgs;
        }
      )
      (
        final: prev:
        let
          goWorkspacePackage = final.callPackage ../../packages/go-workspace-package.nix { };
        in
        {
          gh = final.unstable.gh;
          lldb-mcp-launcher = final.eupkgs.lldb-mcp-launcher;
          ghidra-mcp-headless = final.eupkgs.ghidra-mcp-headless;
          ghidra-mcp = final.callPackage ../../packages/ghidra-mcp.nix {
            inherit (final) ghidra-mcp-headless;
          };
          kanata = prev.kanata;
          kanata-with-cmd = final.kanata.override { withCmd = true; };
          hyper-window-tiling = final.callPackage ../../packages/hyper-window-tiling.nix { };
          hyper-window-tiling-gnome = final.hyper-window-tiling.gnome;
          hyper-window-tiling-kde = final.hyper-window-tiling.kde;
          system-runner = final.writeShellScriptBin "system-runner" (
            builtins.readFile ../../ansible/files/scripts/local/system-runner.sh
          );
          zellij-theme-tools = goWorkspacePackage {
            pname = "zellij-theme-tools";
            subPackages = [ "cmd/zellij-theme-run" ];
            meta = {
              description = "Theme helpers for Zellij and Codex sessions";
              mainProgram = "zellij-theme-run";
              platforms = final.lib.platforms.linux;
            };
          };
        }
      )
    ];
  };
}
