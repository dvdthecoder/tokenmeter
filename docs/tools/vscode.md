# VS Code

VS Code itself doesn't call LLM APIs — its extensions do. tokenmeter supports two of the most popular:

| Extension | Hook |
|---|---|
| Continue.dev | `OPENAI_BASE_URL` env var |
| Cline | Config file patch (coming in v0.4) |

## Continue.dev

Continue reads `OPENAI_BASE_URL` from the environment. After `tokenmeter install`, open a new VS Code window (so it inherits the env var) and Continue will route through tokenmeter automatically.

To verify, run a completion and then:

```sh
tokenmeter query --last 5m
```

## Cline

Cline stores its API endpoint in VS Code settings. Auto-patching is coming in v0.4 via `tokenmeter install --backend vscode`. Until then, manually set:

```json
// VS Code settings.json
{
  "cline.apiProvider": "openai-compatible",
  "cline.openAiBaseUrl": "http://127.0.0.1:4191/v1"
}
```

## GitHub Copilot

GitHub Copilot hardcodes its endpoint (`api.githubcopilot.com`) and does not honour `OPENAI_BASE_URL`. Interception requires a different approach and is tracked in [v0.8](../roadmap.md).
