{
  description = "Mediarium - a self-hosted media manager for movies, TV, music, ebooks and audiobooks";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;

      version = nixpkgs.lib.removeSuffix "\n" (builtins.readFile ./VERSION);

      mkPackage =
        system:
        nixpkgs.legacyPackages.${system}.callPackage ./nix/package.nix {
          inherit version;
          src = self.outPath;
        };

      mkApp = system: {
        type = "app";
        program = "${mkPackage system}/bin/mediarium";
        meta.description = "Mediarium media manager";
      };

      mkNixosModule =
        { pkgs, lib, ... }:
        {
          imports = [ ./nix/module.nix ];
          services.mediarium.package = lib.mkDefault (
            pkgs.callPackage ./nix/package.nix {
              inherit version;
              src = self.outPath;
            }
          );
        };
    in
    {
      packages = forAllSystems (
        system:
        let
          package = mkPackage system;
        in
        {
          default = package;
          mediarium = package;
        }
      );

      apps = forAllSystems (system: {
        default = mkApp system;
        mediarium = mkApp system;
      });

      devShells = forAllSystems (system: {
        default = nixpkgs.legacyPackages.${system}.mkShell {
          packages = with nixpkgs.legacyPackages.${system}; [
            go
            nodejs
            prefetch-npm-deps
            nixfmt
          ];
        };
      });

      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt);

      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          package = self.packages.${system}.default;
          nixos = import ./nix/tests/default.nix {
            inherit pkgs version;
            module = self.nixosModules.default;
          };
        }
      );

      overlays.default = final: _: {
        mediarium = final.callPackage ./nix/package.nix {
          inherit version;
          src = self.outPath;
        };
      };

      nixosModules.default = mkNixosModule;
      nixosModules.mediarium = mkNixosModule;
    };
}
