# Install Tokenmeter

Installs the tokenmeter daemon on this machine and configures detected AI coding tools to route through it.

## Steps

1. Detect OS and architecture
2. Download the latest binary from GitHub releases
3. Run `tokenmeter install` to register as system daemon
4. Patch shell profile with `ANTHROPIC_BASE_URL` and `OPENAI_BASE_URL`
5. Verify traffic is flowing

```bash
# Detect OS/arch and download
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -fsSL "https://github.com/yourorg/tokenmeter/releases/latest/download/tokenmeter-${OS}-${ARCH}" \
  -o /usr/local/bin/tokenmeter && chmod +x /usr/local/bin/tokenmeter

# Install daemon + auto-detect and configure AI tools
tokenmeter install

# Verify
tokenmeter status
```

After install, restart your terminal or run `source ~/.zshrc` (or `~/.bashrc`) for env vars to take effect.
