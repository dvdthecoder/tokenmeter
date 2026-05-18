# tokenmeter

[![CI](https://github.com/dvdthecoder/tokenmeter/actions/workflows/ci.yml/badge.svg)](https://github.com/dvdthecoder/tokenmeter/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Thin, GDPR-compliant token usage proxy for LLM APIs. Sits between your AI coding tools and the model APIs — captures token counts, cost, and latency without ever storing prompt or response content.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/dvdthecoder/tokenmeter/main/scripts/install.sh | sh
tokenmeter install
```

Restart your shell. Every AI tool request is now captured automatically.

## Verify

```sh
tokenmeter verify          # check proxy + all detected tools
tokenmeter query --last 1h # view captured events
```

## Docs

**[dvdthecoder.github.io/tokenmeter](https://dvdthecoder.github.io/tokenmeter)**

## License

Apache 2.0
