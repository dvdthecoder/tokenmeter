# Plugin Registry

Community-contributed plugins for tokenmeter.

## Built-in Providers
| Name | Vendor | Notes | Maintainer |
|---|---|---|---|
| `anthropic` | Anthropic Claude APIs | SSE + REST, all cache tiers | core |
| `openai` | OpenAI + compatible APIs | vLLM, LiteLLM, Ollama, OpenCode | core |
| `gemini` | Google Gemini API | `generativelanguage.googleapis.com` | core |
| `copilot` | GitHub Copilot | HTTPS MITM required; cost always $0 | core |
| `bedrock` | AWS Bedrock | Converse API + InvokeModelWithResponseStream | core |

## Built-in Sinks
| Name | Target | Maintainer |
|---|---|---|
| `sqlite` | Local SQLite file | core |
| `otel` | OTEL Collector (OTLP gRPC) | core |
| `prometheus` | Prometheus scrape endpoint | core |
| `stdout` | stderr (dev/debug) | core |

## Built-in Backends
| Name | Tool | Maintainer |
|---|---|---|
| `claudecode` | Claude Code CLI | core |
| `codex` | OpenAI Codex CLI | core |
| `opencode` | OpenCode TUI | core |
| `vscode` | VS Code (Continue + Cline + Copilot proxy) | core |

---
*To add your plugin, open a PR updating this file and following [CONTRIBUTING.md](CONTRIBUTING.md).*
