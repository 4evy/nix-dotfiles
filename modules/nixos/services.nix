{
  lib,
  pkgs,
  ...
}:
let
  inherit (lib.attrsets) listToAttrs;
  inherit (lib.filesystem) listFilesRecursive;
  inherit (lib.path) hasPrefix;
  inherit (lib.strings) removeSuffix;

  rootfs = ../../spectrum/image/rootfs;
  rootfsSystemdSystem = rootfs + "/usr/lib/systemd/system";
  rootfsSystemdUser = rootfs + "/usr/lib/systemd/user";
  rootfsSystemdSystemConf = rootfs + "/usr/lib/systemd/system.conf.d";
  rootfsSystemdUserConf = rootfs + "/usr/lib/systemd/user.conf.d";

  unitName = path: removeSuffix ".d" (baseNameOf (dirOf path));
  serviceName = path: removeSuffix ".service" (unitName path);

  dropinsFrom =
    paths:
    listToAttrs (
      map (path: {
        name = unitName path;
        value = {
          text = builtins.readFile path;
          overrideStrategy = "asDropin";
        };
      }) paths
    );

  protection = memoryMin: memoryLow: cpuWeight: ioWeight: {
    MemoryMin = memoryMin;
    MemoryLow = memoryLow;
    CPUWeight = cpuWeight;
    IOWeight = ioWeight;
    ManagedOOMPreference = "avoid";
  };

  nativeSystemServices = {
    NetworkManager = protection "64M" "128M" 800 800;
    bluetooth = protection "64M" "128M" 1000 1000;
    dbus-broker = protection "64M" "128M" 1000 1000;
    gdm = protection "128M" "256M" 1000 1000;
    kanata-main = protection "32M" "64M" 1000 1000;
    polkit = protection "64M" "128M" 800 800;
    rtkit-daemon = protection "32M" "64M" 1000 1000;
    systemd-logind = protection "64M" "128M" 1000 1000;
    systemd-udevd = protection "64M" "128M" 800 800;
  };

  nativeUserServices = {
    dbus-broker = protection "128M" "256M" 1000 1000;
    pipewire-pulse = protection "128M" "256M" 1000 1000;
    pipewire = protection "128M" "256M" 1000 1000;
    wireplumber = protection "128M" "256M" 1000 1000;
  };

  nativeUserSlices = [
    "app.slice"
    "background.slice"
    "session.slice"
  ];

  systemUnitFiles = builtins.filter (path: !(hasPrefix rootfsSystemdSystemConf path)) (
    listFilesRecursive rootfsSystemdSystem
  );
  userUnitFiles = builtins.filter (path: !(hasPrefix rootfsSystemdUserConf path)) (
    listFilesRecursive rootfsSystemdUser
  );
  remainingSystemUnitFiles = builtins.filter (
    path: !(builtins.hasAttr (serviceName path) nativeSystemServices) && unitName path != "system.slice"
  ) systemUnitFiles;
  remainingUserUnitFiles = builtins.filter (
    path:
    !(builtins.hasAttr (serviceName path) nativeUserServices)
    && !(builtins.elem (unitName path) nativeUserSlices)
  ) userUnitFiles;
in
{
  environment.etc."uresourced.conf".source = rootfs + "/etc/uresourced.conf";
  fonts.packages = [ pkgs.nerd-fonts.jetbrains-mono ];

  services = {
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

  systemd.oomd.enable = true;

  virtualisation.podman = {
    enable = true;
    dockerCompat = true;
    defaultNetwork.settings.dns_enabled = true;
  };

  systemd = {
    services = lib.mapAttrs (_: serviceConfig: { inherit serviceConfig; }) nativeSystemServices;
    slices.system.sliceConfig = {
      MemoryMin = "512M";
      MemoryLow = "10%";
      ManagedOOMMemoryPressure = "kill";
      ManagedOOMMemoryPressureLimit = "80%";
    };
    units = dropinsFrom remainingSystemUnitFiles;
    user = {
      services = lib.mapAttrs (_: serviceConfig: { inherit serviceConfig; }) nativeUserServices;
      settings.Manager = {
        DefaultMemoryAccounting = true;
        DefaultIOAccounting = true;
        DefaultTasksAccounting = true;
      };
      slices = {
        app.sliceConfig = {
          CPUWeight = 100;
          IOWeight = 100;
          ManagedOOMSwap = "kill";
          ManagedOOMMemoryPressure = "kill";
          ManagedOOMMemoryPressureLimit = "60%";
        };
        background.sliceConfig = {
          CPUWeight = 20;
          IOWeight = 20;
          ManagedOOMSwap = "kill";
          ManagedOOMMemoryPressure = "kill";
          ManagedOOMMemoryPressureLimit = "50%";
        };
        session.sliceConfig = {
          MemoryMin = "768M";
          MemoryLow = "20%";
          CPUWeight = 1000;
          IOWeight = 1000;
          ManagedOOMMemoryPressure = "auto";
        };
      };
      units = dropinsFrom remainingUserUnitFiles;
    };
    settings.Manager = {
      DefaultMemoryAccounting = true;
      DefaultIOAccounting = true;
      DefaultTasksAccounting = true;
    };
  };
}
