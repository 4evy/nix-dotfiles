{
  fetchFromGitLab,
  glib,
  lib,
  meson,
  ninja,
  pipewire,
  pkg-config,
  sourcePin,
  stdenv,
  systemd,
}:

stdenv.mkDerivation {
  pname = "uresourced";
  inherit (sourcePin) version;

  src = fetchFromGitLab {
    domain = "gitlab.freedesktop.org";
    owner = "benzea";
    repo = "uresourced";
    rev = sourcePin.revision;
    hash = sourcePin.source_sha256;
  };

  nativeBuildInputs = [
    glib
    meson
    ninja
    pkg-config
  ];

  buildInputs = [
    glib
    pipewire
    systemd
  ];

  mesonFlags = [
    "-Dappmanagement=true"
    "-Dsystemdsystemunitdir=${placeholder "out"}/lib/systemd/system"
    "-Dsystemduserunitdir=${placeholder "out"}/lib/systemd/user"
  ];

  meta = {
    description = "Dynamically allocate resources to the active graphical user";
    homepage = "https://gitlab.freedesktop.org/benzea/uresourced";
    license = lib.licenses.lgpl21Plus;
    platforms = lib.platforms.linux;
  };
}
