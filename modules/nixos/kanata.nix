{ pkgs, ... }:
{
  services.kanata = {
    enable = true;
    package = pkgs.kanata-with-cmd;
    keyboards.main = {
      configFile = builtins.path {
        path = ../../dotfiles/dot_config/kanata/kanata.kbd;
        name = "kanata-config";
      };
    };
  };
}
