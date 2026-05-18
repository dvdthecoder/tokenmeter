# Backend adapters

A backend adapter detects an installed AI coding tool and patches its configuration to route through tokenmeter.

## Interface

```go
type BackendAdapter interface {
    Name() string
    Detect() bool
    Install() error
    Uninstall() error
    Verify() (bool, error)
}
```

## Scaffold

```sh
tokenmeter scaffold backend cursor
# creates plugins/backends/cursor/cursor.go with stubs
```

## Built-in backends

| Backend | Tool | Status | Hook |
|---|---|---|---|
| `claudecode` | Claude Code CLI | 🔨 v0.4 | `ANTHROPIC_BASE_URL` + skill install |
| `codex` | Codex CLI | 🔨 v0.4 | `OPENAI_BASE_URL` |
| `opencode` | OpenCode | 🔨 v0.4 | `~/.config/opencode/config.json` |
| `vscode` | Continue + Cline | 🔨 v0.4 | VS Code settings JSON |

## Writing a new backend

1. Run `tokenmeter scaffold backend <name>`
2. Implement `Detect()` — check if the tool is installed (`exec.LookPath`, config file exists, etc.)
3. Implement `Install()` — patch the tool's config or shell env to point at `http://127.0.0.1:4191`
4. Implement `Uninstall()` — revert the patch cleanly
5. Implement `Verify()` — confirm traffic is flowing (optional but recommended)
6. Add blank import in `cmd/tokenmeter/main.go`

## Patch strategies

=== "Env var (simplest)"
    Tools that read `OPENAI_BASE_URL` or `ANTHROPIC_BASE_URL` are already handled by `tokenmeter install`'s shell patching. A backend adapter for these tools only needs `Detect()` and `Verify()`.

=== "JSON config file"
    ```go
    func (b *Backend) Install() error {
        path := configPath()
        data, _ := os.ReadFile(path)
        var cfg map[string]any
        json.Unmarshal(data, &cfg)
        cfg["baseURL"] = "http://127.0.0.1:4191"
        modified, _ := json.MarshalIndent(cfg, "", "  ")
        return os.WriteFile(path, modified, 0o644)
    }
    ```

=== "Uninstall (revert)"
    Always store the original value and restore it on uninstall. Do not just delete the key — the tool may need a valid endpoint to function.
