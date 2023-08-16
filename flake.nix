{
  inputs = {
    nixpkgs.url = "flake:nixpkgs/nixpkgs-unstable";
    utils.url = "flake:flake-utils";
  };

  outputs = { self, nixpkgs, utils }: utils.lib.eachDefaultSystem
    (system:
      let
        # enable musl on Linux will trigger a toolchain rebuild
        # making the build very slow
        pkgs = import nixpkgs { inherit system; };
        # if nixpkgs.legacyPackages.${system}.stdenv.hostPlatform.isLinux
        # then nixpkgs.legacyPackages.${system}.pkgsMusl
        # else nixpkgs.legacyPackages.${system};

        crunchy-cli = pkgs.rustPlatform.buildRustPackage.override { stdenv = pkgs.clangStdenv; } rec {
          pname = "crunchy-cli";
          inherit ((pkgs.lib.importTOML ./Cargo.toml).package) version;

          src = pkgs.lib.cleanSource ./.;

          cargoLock = {
            lockFile = ./Cargo.lock;
            allowBuiltinFetchGit = true;
          };

          buildNoDefaultFeatures = true;
          buildFeatures = [ "openssl" ];

          nativeBuildInputs = [
            pkgs.pkg-config
          ] ++ pkgs.lib.optionals pkgs.stdenv.isDarwin [
            pkgs.xcbuild
          ];

          buildInputs = [
            pkgs.openssl
          ] ++ pkgs.lib.optionals pkgs.stdenv.isDarwin [
            pkgs.darwin.Security
          ];
        };
      in
      {
        packages.default = crunchy-cli;

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            cargo
            clippy
            rust-analyzer
            rustc
            rustfmt
          ];

          inputsFrom = builtins.attrValues self.packages.${system};

          buildInputs = [
            pkgs.openssl
            pkgs.libiconv
          ] ++ pkgs.lib.optionals pkgs.stdenv.isDarwin [
            pkgs.darwin.apple_sdk.frameworks.CoreServices
            pkgs.darwin.Security
          ];

          RUST_SRC_PATH = pkgs.rustPlatform.rustLibSrc;
        };

        formatter = pkgs.nixpkgs-fmt;
      }
    ) // {
    overlays.default = final: prev: {
      inherit (self.packages.${final.system}) crunchy-cli;
    };
  };
}
