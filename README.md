# mni

`mni` is the command line client for mNi Cloud.

## Install

### NixOS

```
nix run github:mNi-Cloud/cli
```

```nix
{
  inputs.mni.url = "github:mNi-Cloud/cli";

  outputs = { nixpkgs, mni, ... }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        { environment.systemPackages = [ mni.packages.x86_64-linux.default ]; }
      ];
    };
  };
}
```

### macOS (Homebrew)

```
brew tap mNi-Cloud/tap
brew install mni
```

### Binary download

```
# Linux, x86_64. On arm take mni_Linux_arm64.tar.gz.
curl -L https://github.com/mNi-Cloud/cli/releases/latest/download/mni_Linux_x86_64.tar.gz | tar -xzf - mni && sudo mv mni /usr/local/bin/
```

```
# macOS, Apple silicon. On Intel take mni_Darwin_x86_64.tar.gz.
curl -L https://github.com/mNi-Cloud/cli/releases/latest/download/mni_Darwin_arm64.tar.gz | tar -xzf - mni && sudo mv mni /usr/local/bin/
```

```powershell
# Windows, x86_64. On arm take mni_Windows_arm64.zip.
Invoke-WebRequest https://github.com/mNi-Cloud/cli/releases/latest/download/mni_Windows_x86_64.zip -OutFile mni.zip; Expand-Archive mni.zip -DestinationPath .
```

## Sign in

```
mni login
mni login --context staging --server https://api.example.com --issuer https://anchorage.example.com
```

## Shell completion

```
source <(mni completion zsh)
source <(mni completion bash)
mni completion fish > ~/.config/fish/completions/mni.fish
```

