{
  lib,
  buildGoModule,
  buildNpmPackage,
  fetchFromGitHub,
  makeWrapper,
  nodejs,
  par2cmdline,
  runCommand,
  util-linux,
  _7zz,
  version ? "4.1.0",
  src ? fetchFromGitHub {
    owner = "rdborg";
    repo = "Mediarium";
    rev = "v${version}";
    hash = "sha256-6QJigt4GGI+w0HgfPLYfySDtXu+ZlxSBCuO4oBDev7U=";
  },
}:

let
  # Both the flake source and a fetchFromGitHub checkout unpack to a
  # top-level "source" directory (stdenv's unpacker strips the store hash).
  # The web build lives in web/; the guide generator reaches into ../docs, so
  # the whole repository is the frontend source.
  sourceRoot = "source/web";

  # Mediarium invokes the external tool as "7z". Modern 7-Zip only installs
  # "7zz"; provide the name the app and its test fixtures expect.
  sevenZip = runCommand "mediarium-7z" { } ''
    mkdir -p $out/bin
    ln -s ${_7zz}/bin/7zz $out/bin/7z
  '';

  mediarium-web = buildNpmPackage {
    pname = "mediarium-web";
    inherit version src nodejs;

    inherit sourceRoot;
    npmDepsHash = "sha256-b+x0uXEWXplc2FIyUD9IGGxAq0a35PXiiU3ZIhLUkGo=";

    # buildNpmPackage's default install phase is for a published npm
    # package. Mediarium only needs the compiled SPA, which the Go binary
    # embeds with go:embed.
    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -r dist $out/dist
      runHook postInstall
    '';

    meta = {
      description = "Web frontend for the Mediarium media manager";
      homepage = "https://github.com/rdborg/Mediarium";
      license = lib.licenses.agpl3Only;
      platforms = lib.platforms.linux;
    };
  };
in
buildGoModule {
  pname = "mediarium";
  inherit version src;

  # Run `nix build` with vendorHash = lib.fakeHash after changing go.sum.
  vendorHash = "sha256-CNVVwDsLw9ybDb+EIqOsWeiBaZd6rqwb1jH3n+ulgfc=";

  # Mirrors the release build: no cgo, and the nodynamic tag keeps the AVIF
  # decoder from looking for a system libavif.
  tags = [ "nodynamic" ];
  env.CGO_ENABLED = "0";
  subPackages = [ "cmd/app" ];
  ldflags = [
    "-s"
    "-w"
    "-X"
    "main.version=${version}"
  ];

  nativeBuildInputs = [ makeWrapper ];
  nativeCheckInputs = [
    par2cmdline
    sevenZip
    util-linux
  ];

  # The upstream suite includes timing assertions and local-network torrent
  # tests that are unreliable in a sandbox. Set doCheck = true temporarily
  # when changing the Go code; the release build does not need it. The one
  # timing test that fails on fast localhost transfers stays skipped.
  doCheck = false;
  checkFlags = [
    "-skip"
    "TestATorrentCanHaveItsOwnUploadLimit"
  ];

  preBuild = ''
    rm -rf web/dist
    cp -r ${mediarium-web}/dist web/dist
  '';

  postInstall = ''
    # buildGoModule names the output after the command directory (app);
    # ship it under the name the service and documentation use.
    if [ -e $out/bin/app ]; then
      mv $out/bin/app $out/bin/mediarium
    fi

    install -Dm644 LICENSE $out/share/doc/mediarium/LICENSE
    install -Dm644 THIRD_PARTY_NOTICES.md $out/share/doc/mediarium/THIRD_PARTY_NOTICES.md
  '';

  # The app looks these up in PATH; keep them in the package closure so a
  # plain `nix run` works without the user installing 7z and par2 first.
  postFixup = ''
    wrapProgram $out/bin/mediarium \
      --prefix PATH : ${
        lib.makeBinPath [
          par2cmdline
          sevenZip
        ]
      }
  '';

  passthru = {
    web = mediarium-web;
  };

  meta = {
    description = "Self-hosted media manager for movies, TV shows, music, ebooks and audiobooks";
    longDescription = ''
      Mediarium searches Usenet and torrent indexers, downloads and
      processes releases, and keeps movies, TV shows, music, ebooks and
      audiobooks in a library for Plex, Jellyfin or Emby. It includes
      built-in Usenet and torrent clients and serves its own web UI.
    '';
    homepage = "https://github.com/rdborg/Mediarium";
    changelog = "https://github.com/rdborg/Mediarium/releases";
    license = lib.licenses.agpl3Only;
    mainProgram = "mediarium";
    platforms = lib.platforms.linux;
    sourceProvenance = [ lib.sourceTypes.fromSource ];
  };
}
