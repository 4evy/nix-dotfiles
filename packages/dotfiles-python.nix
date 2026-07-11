{
  lib,
  python314Packages,
}:
python314Packages.buildPythonApplication {
  pname = "dotfiles-python";
  version = "0.1.0";
  pyproject = true;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../pyproject.toml
      ../ansible/files/scripts/workstation
      ../spectrum/scripts/spectrum_build
    ];
  };

  build-system = [ python314Packages.setuptools ];

  # Nixpkgs supplies the backend explicitly and does not resolve build-system
  # requirements from PyPI. Its setuptools 80 is sufficient for this project.
  postPatch = ''
    substituteInPlace pyproject.toml --replace-fail 'setuptools>=83' 'setuptools>=80'
    substituteInPlace pyproject.toml \
      --replace-fail 'httpx-retries>=0.6' 'httpx-retries>=0.5' \
      --replace-fail 'plumbum>=2' 'plumbum>=1.10' \
      --replace-fail 'pydantic>=2.13' 'pydantic>=2.12' \
      --replace-fail 'pydantic-settings>=2.14.2' 'pydantic-settings>=2.12' \
      --replace-fail 'rich>=15' 'rich>=14'
  '';

  dependencies = with python314Packages; [
    astral
    boltons
    defusedxml
    filelock
    githubkit
    humanize
    httpx
    httpx-retries
    jinja2
    packaging
    pillow
    platformdirs
    plumbum
    psutil
    pydantic
    pydantic-settings
    rich
    tenacity
    tomlkit
    typer
  ];

  # These names are supplied by dedicated Nix packages.  Keep this derivation
  # focused on the repository automation commands and avoid buildEnv clashes.
  postInstall = ''
    rm -f "$out/bin/system-runner" "$out/bin/yaml-language-server"
  '';

  doCheck = false;

  meta = {
    description = "Automation commands used by the dotfiles Ansible and chezmoi workflows";
    mainProgram = "dotfiles-scripts";
    platforms = lib.platforms.unix;
  };
}
