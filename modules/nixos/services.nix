{
  lib,
  pkgs,
  ...
}:
let
  inherit (lib.attrsets) listToAttrs;
  inherit (lib.filesystem) listFilesRecursive;
  inherit (lib.path) hasPrefix;
  inherit (lib.strings) removePrefix;

  rootfs = ../../spectrum/image/rootfs;
  rootfsSystemdSystem = rootfs + "/usr/lib/systemd/system";
  rootfsSystemdUser = rootfs + "/usr/lib/systemd/user";
  rootfsSystemdSystemConf = rootfs + "/usr/lib/systemd/system.conf.d";
  rootfsSystemdUserConf = rootfs + "/usr/lib/systemd/user.conf.d";

  relPath = root: path: removePrefix "${toString root}/" (toString path);

  etcFilesFrom =
    sourceRoot: etcPrefix: paths:
    listToAttrs (
      map (path: {
        name = "${etcPrefix}/${relPath sourceRoot path}";
        value.source = path;
      }) paths
    );

  systemUnitFiles = builtins.filter (path: !(hasPrefix rootfsSystemdSystemConf path)) (
    listFilesRecursive rootfsSystemdSystem
  );
  userUnitFiles = builtins.filter (path: !(hasPrefix rootfsSystemdUserConf path)) (
    listFilesRecursive rootfsSystemdUser
  );
in
{
  environment.etc =
    etcFilesFrom rootfsSystemdSystem "systemd/system" systemUnitFiles
    // etcFilesFrom rootfsSystemdUser "systemd/user" userUnitFiles
    // {
      "uresourced.conf".source = rootfs + "/etc/uresourced.conf";
      "systemd/user.conf.d/60-spectrum-resource-accounting.conf".source =
        rootfsSystemdUserConf + "/60-spectrum-resource-accounting.conf";
    };

  services = {
    kmscon = {
      enable = true;
      hwRender = true;
      useXkbConfig = true;
      extraOptions = "--term xterm-256color";
      fonts = [
        {
          name = "JetBrainsMono Nerd Font";
          package = pkgs.nerd-fonts.jetbrains-mono;
        }
      ];
    };

    libinput.enable = true;
    openssh.enable = true;
    xserver.xkb.layout = "us";
  };

  systemd = {
    settings.Manager = {
      DefaultMemoryAccounting = true;
      DefaultIOAccounting = true;
      DefaultTasksAccounting = true;
    };
  };
}
