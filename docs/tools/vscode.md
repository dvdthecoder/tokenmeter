# VS Code

VS Code itself doesn't call LLM APIs — its extensions do. tokenmeter supports two of the most popular:

| Extension | Hook | Auto-configured |
|---|---|---|
| Continue.dev | `OPENAI_BASE_URL` env var | ✅ via shell profile |
| Cline | `settings.json` patch | ✅ via `tokenmeter install` |

## Setup

```sh
tokenmeter install
tokenmeter verify
```

Then open a **new VS Code window** so it inherits the updated environment.

## Continue.dev

Continue reads `OPENAI_BASE_URL` from the environment. After `tokenmeter install` patches your shell profile, open a new VS Code window and Continue will route through tokenmeter automatically.

To confirm:

```sh
# Run a completion in Continue, then:
tokenmeter query --last 5m
```

## Cline

`tokenmeter install` automatically patches `~/.config/Code/User/settings.json` (Linux), `~/Library/Application Support/Code/User/settings.json` (macOS), or `%APPDATA%\Code\User\settings.json` (Windows) with:

```json
{
  "cline.apiProvider": "openai-compatible",
  "cline.openAiBaseUrl": "http://127.0.0.1:4191/v1"
}
```

Existing settings are preserved — only the Cline keys are added or updated.

To target VS Code only:

```sh
tokenmeter install --backend vscode
```

To revert:

```sh
tokenmeter uninstall   # removes cline.* keys from settings.json
```

## GitHub Copilot

GitHub Copilot hardcodes `api.githubcopilot.com` and does not honour `OPENAI_BASE_URL`. Interception requires a different approach and is tracked in [issue #13](https://github.com/dvdthecoder/tokenmeter/issues/13) (v0.8).
