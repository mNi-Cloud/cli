# mni

`mni` is the command line client for mNi Cloud.

## AI agent skill

Install the bundled skill for Codex at user scope:

```sh
gh skill install mNi-Cloud/cli mni-cli@main --agent codex --scope user
```

Omit `--scope user` to install it only for the current project. GitHub Copilot,
Claude Code, Cursor, Gemini CLI, and other supported agents can be selected with
`--agent`. After a tagged release includes the skill, the `@main` version may be
omitted to install that release.

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
$dir = "$env:LOCALAPPDATA\Programs\mNi\mni"
Invoke-WebRequest https://github.com/mNi-Cloud/cli/releases/latest/download/mni_Windows_x86_64.zip -OutFile "$env:TEMP\mni.zip"; Expand-Archive "$env:TEMP\mni.zip" -DestinationPath $dir -Force
[Environment]::SetEnvironmentVariable('Path', "$([Environment]::GetEnvironmentVariable('Path', 'User'));$dir", 'User')
```

Open a new PowerShell window before running `mni`.

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
