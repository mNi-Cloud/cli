{
  description = "mNi Cloud CLI";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";

  outputs = { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forEachSystem = f:
        builtins.listToAttrs (map (system: {
          name = system;
          value = f system;
        }) systems);

      # A flake build has no git checkout to run `git describe` in, so the
      # revision it was evaluated from stands in for the tag the Makefile uses.
      version = self.shortRev or self.dirtyShortRev or "unknown";
    in {
      packages = forEachSystem (system:
        let pkgs = import nixpkgs { inherit system; }; in {
          default = (pkgs.buildGoModule.override { go = pkgs.go_1_24; }) {
            pname = "mni";
            inherit version;
            src = ./.;
            vendorHash = "sha256-p6exn2bQlDtI0PDXIuj5WvArbCChEj6anEhcNBAuwmU=";

            subPackages = [ "cmd" ];
            ldflags = [ "-s" "-w" "-X main.version=${version}" ];

            # buildGoModule names a binary after its directory, and this one
            # lives in cmd/ like every other mNi repository.
            postInstall = ''
              mv "$out/bin/cmd" "$out/bin/mni"
            '';

            meta = {
              description = "CLI client for mNi Cloud";
              mainProgram = "mni";
            };
          };
        });

      devShells = forEachSystem (system:
        let pkgs = import nixpkgs { inherit system; }; in {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go_1_24
              gopls
              gotools
              golangci-lint
              gnumake
              git
            ];

            shellHook = ''
              echo "mNi Cloud cli devShell: go $(go version | awk '{ print $3 }')"
              echo "Try: make build  |  make test  |  make lint"
            '';
          };
        });
    };
}
