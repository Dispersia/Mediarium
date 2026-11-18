{
  pkgs,
  module,
  version,
}:
pkgs.testers.nixosTest {
  name = "mediarium";

  nodes.machine =
    { pkgs, ... }:
    {
      imports = [ module ];

      services.mediarium = {
        enable = true;
        openFirewall = true;
      };

      environment.systemPackages = [ pkgs.curl ];
      system.stateVersion = "26.11";
    };

  testScript = ''
    start_all()

    machine.wait_for_unit("mediarium.service")
    machine.wait_for_open_port(8264)

    # The binary is the one from this flake, the embedded frontend is served,
    # and the config folder was created with the database and encryption key.
    machine.succeed("curl -fsS http://127.0.0.1:8264/api/version | grep -F '${version}'")
    machine.wait_until_succeeds("test -f /var/lib/mediarium/config/app.db")
    machine.wait_until_succeeds("test -f /var/lib/mediarium/config/secret.key")

    # systemd is the supervisor, so the restart controls must recognise it.
    machine.succeed("systemctl show -p ActiveState --value mediarium.service | grep -qx active")
  '';
}
