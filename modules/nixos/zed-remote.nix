{ lib, pkgs, ... }:
{
  systemd.tmpfiles.rules =
    let
      inherit (pkgs.zed-editor.remote_server) version;
      binaryName = "zed-remote-server-stable-${version}";
    in
    [
      "d /home/4evy/.zed_server 0755 4evy users - -"
      "L+ /home/4evy/.zed_server/${binaryName} - - - - ${lib.meta.getExe' pkgs.zed-editor.remote_server binaryName}"
    ];
}
