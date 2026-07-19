{
  lib,
  pkgs,
  ...
}:
let
  # Import the rootfs once. Taking each local file as an independent Nix path
  # would create another single-file store object for every shared policy.
  rootfs = builtins.path {
    path = ../../spectrum/image/rootfs;
    name = "spectrum-rootfs";
  };

  # NixOS merges units from systemd.packages before adding its generated unit
  # overrides. Keep Spectrum's checked-in drop-ins as real files so both
  # operating systems consume the same policy rather than translating it into
  # a second Nix representation.
  spectrumSystemdUnits = pkgs.runCommandLocal "spectrum-systemd-units" { } ''
    mkdir -p "$out/lib/systemd"
    ln -s ${rootfs}/usr/lib/systemd/system "$out/lib/systemd/system"
    ln -s ${rootfs}/usr/lib/systemd/user "$out/lib/systemd/user"
  '';
in
{
  environment.etc = {
    # BlueZ's NixOS module normally generates an empty input.conf. Spectrum's
    # source file intentionally replaces that default.
    "bluetooth/input.conf".source = lib.mkForce (rootfs + "/etc/bluetooth/input.conf");
    "modprobe.d/60-spectrum-bluetooth.conf".source =
      rootfs + "/usr/lib/modprobe.d/60-spectrum-bluetooth.conf";
    "systemd/system.conf.d/60-spectrum-resource-accounting.conf".source =
      rootfs + "/usr/lib/systemd/system.conf.d/60-spectrum-resource-accounting.conf";
    "systemd/user.conf.d/60-spectrum-resource-accounting.conf".source =
      rootfs + "/usr/lib/systemd/user.conf.d/60-spectrum-resource-accounting.conf";
    "uresourced.conf".source = rootfs + "/etc/uresourced.conf";
  };

  fonts.packages = [ pkgs.nerd-fonts.jetbrains-mono ];

  services = {
    # The system daemon still supplies ancestor allocations to whichever user
    # owns the active graphical session. Spectrum's shared user-unit drop-in
    # keeps the separate --user app-management daemon disabled by default.
    dbus.packages = [ pkgs.uresourced ];
    flatpak.enable = true;

    kmscon = {
      enable = true;
      useXkbConfig = true;
      extraOptions = "--term xterm-256color";
      config.hwaccel = true;
      config."font-name" = "JetBrainsMono Nerd Font";
    };

    libinput.enable = true;
    openssh.enable = true;
    pcscd.enable = true;
    xserver.xkb.layout = "us";
  };

  programs.gnupg.agent = {
    enable = true;
    enableSSHSupport = false;
  };

  systemd = {
    oomd.enable = true;
    packages = [
      pkgs.uresourced
      spectrumSystemdUnits
    ];
    services."user@".wants = [ "uresourced.service" ];
  };

  virtualisation.podman = {
    enable = true;
    dockerCompat = true;
    defaultNetwork.settings.dns_enabled = true;
  };
}
