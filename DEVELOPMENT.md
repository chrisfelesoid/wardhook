# Development

# Prerequisites

- [`mise`](https://mise.jdx.dev/) (manages Go and lint tools per `.mise.toml`)


## Clone and install tools

```bash
git clone https://github.com/chrisfelesoid/wardhook.git
cd wardhook
mise install
```


## Common commands

```bash
mise run pre-commit          # secrets / lint / format checks
mise run test                # unit, integration, and E2E tests
mise lint:golangci-lint      # Golang static analysis
mise run build               # cross-compile to ./bin/ (5 targets)
```

# Recomendation

Claude Code
```
/plugin marketplace add obra/superpowers-marketplace
/plugin install superpowers@superpowers-marketplace
```


# Hook Specifications

## Claude Code

https://code.claude.com/docs/en/hooks

`package/sdk.d.ts`
https://github.com/anthropics/claude-agent-sdk-typescript/releases/tag/v0.3.169


## Codex

input:
https://github.com/openai/codex/blob/rust-v0.138.0/codex-rs/hooks/schema/generated/pre-tool-use.command.input.schema.json

output:
https://github.com/openai/codex/blob/rust-v0.138.0/codex-rs/hooks/schema/generated/pre-tool-use.command.output.schema.json


## Cursor

https://cursor.com/docs/hooks.md


## GitHub Copilot

https://github.com/microsoft/vscode-docs/blob/main/docs/agent-customization/hooks.md


## Gemini CLI

**Deprecated for personal**

`interface HookInput`, `interface HookOutput`
https://github.com/google-gemini/gemini-cli/blob/v0.45.2/packages/core/src/hooks/types.ts


## Antigravity

https://antigravity.google/docs/hooks
