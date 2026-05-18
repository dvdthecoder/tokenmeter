# Installation

## One-line install (macOS / Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/dvdthecoder/tokenmeter/main/scripts/install.sh | sh
```

This downloads the right binary for your platform, places it in `/usr/local/bin`, then runs `tokenmeter install`.

## Manual install

```sh
# macOS arm64
curl -fsSL https://github.com/dvdthecoder/tokenmeter/releases/latest/download/tokenmeter-darwin-arm64 \
  -o /usr/local/bin/tokenmeter && chmod +x /usr/local/bin/tokenmeter

# macOS amd64
curl -fsSL https://github.com/dvdthecoder/tokenmeter/releases/latest/download/tokenmeter-darwin-amd64 \
  -o /usr/local/bin/tokenmeter && chmod +x /usr/local/bin/tokenmeter

# Linux amd64
curl -fsSL https://github.com/dvdthecoder/tokenmeter/releases/latest/download/tokenmeter-linux-amd64 \
  -o /usr/local/bin/tokenmeter && chmod +x /usr/local/bin/tokenmeter
```

## Build from source

```sh
git clone https://github.com/dvdthecoder/tokenmeter
cd tokenmeter
make build          # → bin/tokenmeter
make install        # build + tokenmeter install
```

Requires Go 1.23+.

## `tokenmeter install`

After placing the binary, run:

```sh
tokenmeter install
```

This does three things:

1. Writes a default config to `~/.config/tokenmeter/config.yaml`
2. Registers the daemon as a system service (launchd on macOS, systemd user unit on Linux)
3. Patches your shell profile (`~/.zshrc`, `~/.bashrc`, or `~/.config/fish/conf.d/tokenmeter.fish`) with:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:4191
export OPENAI_BASE_URL=http://127.0.0.1:4191
```

Restart your shell (or `source ~/.zshrc`) and tokenmeter will intercept all traffic automatically.

## Verify

```sh
tokenmeter status
# status:  running (pid 12345)
# log:     ~/Library/Application Support/tokenmeter/tokenmeter.log
```

## Uninstall

```sh
tokenmeter uninstall
```

Stops the daemon, removes the service registration, and strips the env vars from your shell profile. Your SQLite database is left in place — run `tokenmeter purge --retention-days 0` first if you want to wipe it.
