{
  attrValues,
  isx86_64,
  lib,
  mkIf,
  pkgs,
}:
{
  # General hardware configuration
  environment.systemPackages = attrValues { inherit (pkgs) libva-utils; };
  environment.sessionVariables = {
    # It tells supported apps to use the Ozone/Wayland backend
    NIXOS_OZONE_WL = "1";

    # Disables the RDD (Remote Data Decoder) sandbox in Firefox.
    MOZ_DISABLE_RDD_SANDBOX = "1";

    # Hardware cursors are currently broken on wlroots
    WLR_NO_HARDWARE_CURSORS = "1";

    # Improve compatibility for older Java GUI (AWT/Swing) apps, especially on
    # non-reparenting WMs (most Wayland compositors, some X11 WMs)
    _JAVA_AWT_WM_NONREPARENTING = "1";

    # Enable automatic scaling for Qt5/Qt6 applications based on monitor DPI
    # Useful for HiDPI displays
    QT_AUTO_SCREEN_SCALE_FACTOR = "1";

    # Specifies the platform to use for EGL (OpenGL ES) applications.
    # Setting this to "wayland" ensures that EGL-based apps use the Wayland backend.
    EGL_PLATFORM = "wayland";

    # Enable Variable Refresh Rate (VRR/FreeSync) for OpenGL and GLX
    __GL_VRR_ALLOWED = "1";
    __GLX_VRR_ALLOWED = "1";
  };

  hardware.graphics = {
    enable = true;
    enable32Bit = mkIf isx86_64 true;
    extraPackages = attrValues {
      inherit (pkgs)
        libva-vdpau-driver
        libvdpau-va-gl
        mesa
        vulkan-loader
        ;
    };
  }
  // lib.attrsets.optionalAttrs isx86_64 {
    extraPackages32 = attrValues {
      inherit (pkgs.pkgsi686Linux) libva-vdpau-driver libvdpau-va-gl mesa;
    };
  };

  # Spectrum installs Solaar and its udev support for the shared Logitech
  # rules and user service. Use the NixOS hardware module for the same
  # package and device permissions.
  hardware.logitech.wireless = {
    enable = true;
    enableGraphical = true;
  };
}
