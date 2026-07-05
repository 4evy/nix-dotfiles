{ inputs, ... }:
{
  imports = [
    ../cross
    ./desktop-packages.nix
    ./gnome.nix
    ./ghidra-mcp.nix
    ./hardware.nix
    ./kanata.nix
    ./kde.nix
    ./networking.nix
    ./packages.nix
    ./shell.nix
    inputs.nixcord.nixosModules.nixcord
    ./nixcord/settings.nix
    ./services.nix
    ./zed-remote.nix
  ];

  programs.nixcord.user = "4evy";
}
