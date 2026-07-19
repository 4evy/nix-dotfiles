{
  pkgs,
  lib,
  config,
  ...
}:
let
  inherit (lib.attrsets) attrValues;
  inherit (lib.modules) mkIf mkMerge;
  inherit (lib.options) mkEnableOption;
  inherit (config.nixpkgs.hostPlatform) isx86_64;
in
{
  options.local.nvidia.enable = mkEnableOption "NVIDIA";
  options.local.amd.enable = mkEnableOption "AMD";

  config = mkMerge [
    (import ./config.nix {
      inherit
        attrValues
        isx86_64
        lib
        mkIf
        pkgs
        ;
    })
    (mkIf config.local.nvidia.enable {
      # nixpkgs.config.cudaSupport = true;
      boot.extraModprobeConfig =
        "options nvidia "
        + lib.strings.concatStringsSep " " [
          # NVIDIA assumes that by default your CPU doesn't support `PAT`, but this
          # is effectively never the case
          "NVreg_UsePageAttributeTable=1"
          # This is sometimes needed for ddc/ci support, see
          # https://www.ddcutil.com/nvidia/
          "NVreg_RegistryDwords=RMUseSwI2c=0x01;RMI2cSpeed=100"
        ];

      environment.sessionVariables = {
        # Required to run the correct GBM backend for NVIDIA GPUs on Wayland
        GBM_BACKEND = "nvidia-drm";
        # Apparently, without this NOUVEAU may attempt to be used instead
        # (despite it being blacklisted)
        __GLX_VENDOR_LIBRARY_NAME = "nvidia";

        NVD_BACKEND = "direct";
        LIBVA_DRIVER_NAME = "nvidia";
      };

      services.xserver.videoDrivers = [ "nvidia" ];

      hardware = {
        nvidia = {
          open = true;
          package = config.boot.kernelPackages.nvidiaPackages.latest;
          modesetting.enable = true;
          powerManagement.enable = true;
          powerManagement.finegrained = true;
        };

        graphics.extraPackages = attrValues {
          inherit (pkgs) nv-codec-headers-12;
        };
      };
    })
    (mkIf config.local.amd.enable {
      # HIP libraries support - many applications hard-code HIP library paths
      systemd.tmpfiles.rules =
        let
          rocmEnv = pkgs.symlinkJoin {
            name = "rocm-combined";
            paths = attrValues {
              inherit (pkgs.pkgs.rocmPackages) rocblas hipblas clr;
            };
          };
        in
        [ "L+    /opt/rocm   -    -    -     -    ${rocmEnv}" ];

      hardware.graphics.extraPackages = attrValues {
        inherit (pkgs.rocmPackages) clr;
      };
    })
  ];
}
