{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.mediarium;

  envValue = value: if builtins.isBool value then (if value then "1" else "0") else toString value;

  environment =
    lib.mapAttrs (_: envValue) (lib.filterAttrs (_: value: value != null) cfg.environment)
    // {
      APP_PORT = toString cfg.port;
      CONFIG_DIR = toString cfg.configDir;
      DOWNLOADS_DIR = toString cfg.downloadsDir;
      MOVIES_DIR = toString cfg.moviesDir;
      TV_DIR = toString cfg.tvDir;
      MUSIC_DIR = toString cfg.musicDir;
      EBOOKS_DIR = toString cfg.ebooksDir;
      AUDIOBOOKS_DIR = toString cfg.audiobooksDir;
      TRUSTED_PROXIES = cfg.trustedProxies;
      ALLOWED_ORIGINS = cfg.allowedOrigins;
      HOME = "/var/lib/mediarium";
    }
    // lib.optionalAttrs (cfg.urlBase != "") {
      URL_BASE = cfg.urlBase;
    }
    // lib.optionalAttrs cfg.pauseAutomation {
      MEDIARIUM_PAUSE_AUTOMATION = "1";
    }
    // lib.optionalAttrs cfg.disableSSO {
      MEDIARIUM_DISABLE_SSO = "1";
    }
    // lib.optionalAttrs cfg.disableStats {
      MEDIARIUM_DISABLE_STATS = "1";
    };

  mediaDirs = [
    cfg.downloadsDir
    cfg.moviesDir
    cfg.tvDir
    cfg.musicDir
    cfg.ebooksDir
    cfg.audiobooksDir
  ];

  # Only create the folders that live under the service's own data
  # directory. A folder on a mounted disk (for example /mnt/media/movies)
  # is the administrator's to create; creating it on the root filesystem
  # while the disk is absent would hide the real one when it is mounted.
  managedMediaDirs = lib.filter (
    dir: lib.hasPrefix "${toString cfg.dataDir}/" (toString dir)
  ) mediaDirs;

  tmpfilesUser = "${cfg.user} ${cfg.group}";
in
{
  options.services.mediarium = {
    enable = lib.mkEnableOption "Mediarium, a self-hosted media manager";

    package = lib.mkPackageOption pkgs "mediarium" { };

    user = lib.mkOption {
      type = lib.types.str;
      default = "mediarium";
      description = "User account that runs Mediarium and owns its config, data and media folders.";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "mediarium";
      description = "Group that runs Mediarium and owns its config, data and media folders.";
    };

    createUser = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = ''
        Whether to create the {option}`user` and {option}`group`. Set this to
        false when both already exist (for example a normal login account used
        to own an existing library).
      '';
    };

    extraGroups = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      description = "Extra groups for the Mediarium user, for media folders owned by another group.";
    };

    configDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/mediarium/config";
      description = "Directory for app.db and secret.key. Back it up as one unit.";
    };

    dataDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/mediarium/data";
      description = ''
        Parent directory for the default media folders. Keep downloads,
        movies and TV shows on the same filesystem so imports can hardlink
        instead of copy.
      '';
    };

    downloadsDir = lib.mkOption {
      type = lib.types.path;
      default = "${config.services.mediarium.dataDir}/downloads";
      description = "Folder for downloads and incomplete downloads.";
    };

    moviesDir = lib.mkOption {
      type = lib.types.path;
      default = "${config.services.mediarium.dataDir}/movies";
      description = "Folder for the movie library.";
    };

    tvDir = lib.mkOption {
      type = lib.types.path;
      default = "${config.services.mediarium.dataDir}/tv";
      description = "Folder for the TV library.";
    };

    musicDir = lib.mkOption {
      type = lib.types.path;
      default = "${config.services.mediarium.dataDir}/music";
      description = "Folder for the music library. Only used after the music module is switched on.";
    };

    ebooksDir = lib.mkOption {
      type = lib.types.path;
      default = "${config.services.mediarium.dataDir}/ebooks";
      description = "Folder for the ebooks library. Only used after the ebooks module is switched on.";
    };

    audiobooksDir = lib.mkOption {
      type = lib.types.path;
      default = "${config.services.mediarium.dataDir}/audiobooks";
      description = "Folder for the audiobooks library. Only used after the audiobooks module is switched on.";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 8264;
      description = "Port the web UI and API listen on.";
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open {option}`port` in the NixOS firewall.";
    };

    torrentPort = lib.mkOption {
      type = lib.types.port;
      default = 58264;
      description = ''
        The torrent listen port. This is only used for the firewall and for
        documentation; change the port in Mediarium's own settings as well.
      '';
    };

    openTorrentPort = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open {option}`torrentPort` (TCP and UDP) in the NixOS firewall.";
    };

    urlBase = lib.mkOption {
      type = lib.types.str;
      default = "";
      example = "/mediarium";
      description = "Sub-path Mediarium is served under behind a reverse proxy. Empty means the root.";
    };

    trustedProxies = lib.mkOption {
      type = lib.types.str;
      default = "private";
      description = ''
        Reverse proxies whose X-Forwarded-* headers are trusted. "private"
        trusts loopback and private ranges, "none" trusts none, or give a
        comma-separated list of addresses and CIDR ranges.
      '';
    };

    allowedOrigins = lib.mkOption {
      type = lib.types.str;
      default = "";
      description = "Extra host names accepted as the origin of state-changing browser requests.";
    };

    pauseAutomation = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Start in safe mode: no automatic searching, downloading, refreshing or connection checks.";
    };

    disableSSO = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Disable OpenID Connect and LDAP sign-in and turn password sign-in on, as a way back in.";
    };

    disableStats = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Never send anonymous usage statistics, whatever the saved setting says.";
    };

    environment = lib.mkOption {
      type = lib.types.attrsOf (
        lib.types.nullOr (
          lib.types.oneOf [
            lib.types.str
            lib.types.int
            lib.types.bool
          ]
        )
      );
      default = { };
      example = {
        TMDB_API_KEY = "...";
      };
      description = ''
        Extra environment variables for Mediarium. Null values are omitted;
        booleans become 1 and 0. Values that are not set here are unset, so
        secrets belong in {option}`environmentFile` rather than the Nix store.
      '';
    };

    environmentFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      example = "/run/secrets/mediarium.env";
      description = ''
        Optional systemd EnvironmentFile, read as root before the service drops
        privileges. Use it for secrets or values you do not want copied into
        the world-readable Nix store.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    users.users = lib.optionalAttrs cfg.createUser {
      ${cfg.user} = {
        isSystemUser = true;
        group = cfg.group;
        extraGroups = cfg.extraGroups;
        home = "/var/lib/mediarium";
        createHome = false;
        description = "Mediarium media manager";
      };
    };

    users.groups = lib.optionalAttrs cfg.createUser {
      ${cfg.group} = { };
    };

    systemd.tmpfiles.rules = [
      "d '${cfg.configDir}' 0750 ${tmpfilesUser} -"
      "d '${cfg.dataDir}' 0750 ${tmpfilesUser} -"
    ]
    ++ map (dir: "d '${dir}' 0750 ${tmpfilesUser} -") managedMediaDirs;

    systemd.services.mediarium = {
      description = "Mediarium media manager";
      documentation = [ "https://github.com/rdborg/Mediarium" ];
      wantedBy = [ "multi-user.target" ];
      wants = [ "network-online.target" ];
      after = [ "network-online.target" ];
      unitConfig.RequiresMountsFor = mediaDirs;

      inherit environment;
      path = [
        cfg.package
        pkgs.par2cmdline
      ];

      serviceConfig = {
        Type = "simple";
        User = cfg.user;
        Group = cfg.group;
        ExecStart = "${cfg.package}/bin/mediarium";
        Restart = "always";
        RestartSec = 5;
        TimeoutStopSec = 20;

        StateDirectory = "mediarium";
        StateDirectoryMode = "0750";

        # ReadWritePaths uses the "-" prefix for the media folders: a missing
        # external mount must not keep the whole service from starting.
        ReadWritePaths = [
          cfg.configDir
          cfg.dataDir
        ]
        ++ map (dir: "-${toString dir}") mediaDirs;

        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        RestrictSUIDSGID = true;
        LockPersonality = true;
      }
      // lib.optionalAttrs (cfg.environmentFile != null) {
        EnvironmentFile = cfg.environmentFile;
      };
    };

    networking.firewall.allowedTCPPorts =
      lib.optionals cfg.openFirewall [ cfg.port ]
      ++ lib.optionals cfg.openTorrentPort [ cfg.torrentPort ];
    networking.firewall.allowedUDPPorts = lib.optionals cfg.openTorrentPort [ cfg.torrentPort ];
  };
}
