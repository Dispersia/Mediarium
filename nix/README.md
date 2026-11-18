# Mediarium on Nix and NixOS

This directory has the packaging needed to build and deploy Mediarium with
Nix, plus the same files in the shape nixpkgs expects.

## What the flake provides

| Output | What it is |
|---|---|
| `packages.<system>.default` / `.mediarium` | The release binary, with the web UI built and embedded, `par2` and 7-Zip in its runtime `PATH`. |
| `nixosModules.default` / `.mediarium` | The `services.mediarium` NixOS module, pre-wired to the package from this flake. |
| `overlays.default` | Adds `pkgs.mediarium` to a nixpkgs instance. |
| `checks.<system>.package` | Builds the package. |
| `checks.<system>.nixos` | Boots a VM, starts the service and checks the API, database and key. |
| `devShells.default` | Go, Node.js, npm dependency prefetching and `nixfmt`. |
| `apps.default` | `nix run` entry point. |

Only `x86_64-linux` and `aarch64-linux` are listed. The application is a
Linux service; the Docker image is the supported way to run it elsewhere.

## Quick check

```bash
nix build .#mediarium
./result/bin/mediarium --version-check

# The same binary with par2 and 7z on PATH:
nix run .# -- --version-check
```

The binary expects `/config` by default. For a real deployment use the NixOS
module (or set `CONFIG_DIR`, `DOWNLOADS_DIR`, `MOVIES_DIR` and `TV_DIR`).

## NixOS

Add the flake as an input:

```nix
{
  inputs.mediarium.url = "github:rdborg/Mediarium";

  outputs = { self, nixpkgs, mediarium, ... }: {
    nixosConfigurations.media = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        mediarium.nixosModules.default
        {
          services.mediarium = {
            enable = true;
            openFirewall = true;

            # Put these on the same filesystem so imports hardlink.
            downloadsDir = "/srv/media/downloads";
            moviesDir = "/srv/media/movies";
            tvDir = "/srv/media/tv";

            # Optional media types; the defaults live under dataDir.
            musicDir = "/srv/media/music";
            ebooksDir = "/srv/media/ebooks";
            audiobooksDir = "/srv/media/audiobooks";

            # If the torrent client is used and the port is reachable:
            # openTorrentPort = true;
          };
        }
      ];
    };
  };
}
```

Then:

```bash
sudo nixos-rebuild switch --flake .#media
```

Open `http://<host>:8264` and follow the setup wizard.

### Defaults

- Service user and group: `mediarium`.
- Config: `/var/lib/mediarium/config` (`app.db`, `secret.key`, update
  markers). Back this up as one unit.
- Data: `/var/lib/mediarium/data`, with `downloads`, `movies`, `tv`, `music`,
  `ebooks` and `audiobooks` underneath.
- Web port: `8264`.
- The service waits for the mounts holding the media folders
  (`RequiresMountsFor`) and only gets write access to the configured folders.
- The app's own restart controls work because systemd is detected.

The media folders are not created when they are outside `dataDir`: create
them (and give the service user write access) before starting, especially
when they are mount points. Folders inside `dataDir` are created
automatically.

### Options

Run `nixos-option services.mediarium` or read `nix/module.nix`. The important
ones:

| Option | Default | Purpose |
|---|---|---|
| `enable` | `false` | Turn the service on. |
| `package` | `pkgs.mediarium` | Package to run. The flake module points this at its own build. |
| `user`, `group` | `mediarium` | Account and group. |
| `createUser` | `true` | Create the user and group. Set `false` for an existing account. |
| `extraGroups` | `[]` | Extra groups for media folders owned by another group. |
| `configDir` | `/var/lib/mediarium/config` | Database, key and update folder. |
| `dataDir` | `/var/lib/mediarium/data` | Parent of the default media folders. |
| `downloadsDir` | `dataDir/downloads` | Download and incomplete folders. |
| `moviesDir` | `dataDir/movies` | Movie library. |
| `tvDir` | `dataDir/tv` | TV library. |
| `musicDir`, `ebooksDir`, `audiobooksDir` | under `dataDir` | Optional media libraries. |
| `port` | `8264` | Web UI and API port. |
| `openFirewall` | `false` | Open `port` in the NixOS firewall. |
| `torrentPort` | `58264` | Torrent listen port, for documentation and the firewall. |
| `openTorrentPort` | `false` | Open `torrentPort` for TCP and UDP. |
| `urlBase` | `""` | Sub-path behind a reverse proxy, for example `/mediarium`. |
| `trustedProxies` | `"private"` | Proxies whose `X-Forwarded-*` headers are trusted. |
| `allowedOrigins` | `""` | Extra hosts accepted as browser request origins. |
| `pauseAutomation` | `false` | Start in safe mode. |
| `disableSSO` | `false` | Turn OIDC/LDAP off and password sign-in on. |
| `disableStats` | `false` | Never send anonymous usage statistics. |
| `environment` | `{}` | Extra environment variables. Null values are omitted; booleans become `1`/`0`. |
| `environmentFile` | `null` | systemd `EnvironmentFile` for secrets or values kept out of the Nix store. |

### Secrets

Do not put API keys or passwords in `environment`: values in a Nix
configuration end up in the world-readable Nix store (and often in the
system closure). Use `environmentFile` with a root-readable runtime file:

```nix
services.mediarium.environmentFile = "/run/secrets/mediarium.env";
```

The file uses the normal systemd syntax, one `NAME=value` per line. The app
also accepts `NAME_FILE=/path` for its own secret-file convention.

### Reverse proxy and sub-path

```nix
services.mediarium = {
  urlBase = "/mediarium";
  trustedProxies = "127.0.0.1,::1,10.0.0.0/8";
};
```

Point the proxy at `http://127.0.0.1:8264` and pass `Host`,
`X-Forwarded-For`, `X-Forwarded-Proto` and `X-Forwarded-Host`. If the proxy
rewrites `Host`, set `allowedOrigins` to the public host name.

## Updating hashes

The package has three fixed-output hashes:

- `src.hash` (GitHub source tarball),
- `vendorHash` (Go modules from `go.sum`),
- `npmDepsHash` (web dependencies from `web/package-lock.json`).

After bumping `VERSION` and `version` in `nix/package.nix`:

```bash
# Go modules
# Set vendorHash = lib.fakeHash;, build, and copy the "got:" hash.
nix build .#mediarium

# npm dependencies
nix shell nixpkgs#prefetch-npm-deps -c prefetch-npm-deps web/package-lock.json

# Source tarball (for the fetchFromGitHub default in package.nix)
nix-prefetch-url --unpack \
  "https://github.com/rdborg/Mediarium/archive/vX.Y.Z.tar.gz" \
  | xargs nix hash convert --to sri --hash-algo sha256
```

The flake builds from the local tree (`src = self.outPath`), so the source
hash is only used when `nix/package.nix` is copied into nixpkgs. The version
in `nix/package.nix` must still match the root `VERSION` file.

## Putting it in nixpkgs

The files are arranged for the modern nixpkgs layout:

| This repository | nixpkgs |
|---|---|
| `nix/package.nix` | `pkgs/by-name/me/mediarium/package.nix` |
| `nix/module.nix` | `nixos/modules/services/misc/mediarium.nix` |
| `nix/tests/default.nix` | `nixos/tests/mediarium.nix` |

`nix/package.nix` already defaults to `fetchFromGitHub` at
`v${version}`, so it builds without the flake. In nixpkgs:

1. Add the module to `nixos/modules/module-list.nix`.
2. Change the test's `imports = [ module ]` to the module path (or just set
   `services.mediarium.enable = true` after it is registered).
3. Add a maintainer to `meta.maintainers` (none is guessed here).
4. Run `nix-build -A mediarium` and `nix-build nixos/tests/mediarium.nix`.

The package's test suite is disabled by default because it contains timing
assertions and local torrent transfer tests that are unreliable in a build
sandbox. To run it while changing Go code, set `doCheck = true` in
`nix/package.nix` (or override the package) and run:

```bash
nix build .#mediarium
```

The one timing-specific upload-limit test is skipped; the rest runs with
`par2`, 7-Zip and `setpriv` available.
