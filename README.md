# wardhook

English | [日本語](README.ja.md)

A configurable PreToolUse hook filter for AI Coding Agents.
It blocks destructive actions that `settings.json` permissions cannot catch, while allowing safe operations.


[![License](https://img.shields.io/github/license/chrisfelesoid/wardhook)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/chrisfelesoid/wardhook)](go.mod)
[![Release](https://img.shields.io/github/v/release/chrisfelesoid/wardhook)](https://github.com/chrisfelesoid/wardhook/releases)

## Features

- **Flag-order independent matching**: `flags_all: [r, f]` matches `-rf` / `-fr` / `-r -f` regardless of order.
- **Shell metacharacter decomposition**: `;`, `&&`, `||`, `|`, `$()`, and backticks are split into sub-commands and each evaluated.
- **Recursive wrapper expansion**: `bash -c "..."` and `gcloud compute ssh ... -- ...` are re-parsed so rules apply to the nested command.
- **Cross-tool path globs**: a single `tool: "*"` rule blocks `.env`-style files across `Read` / `Write` / `Edit` / `Bash`.
- **`except` clauses**: allow writes under `/tmp/**` while denying others.
- **Fail-closed**: parse errors and unknown structures fall back to `ask`, not `allow`.

## Install

```bash
# Example: Linux amd64
curl -L -o /tmp/wh.tar.gz \
  https://github.com/chrisfelesoid/wardhook/releases/latest/download/wardhook_<version>_linux_amd64.tar.gz
tar -xzf /tmp/wh.tar.gz -C /tmp/
sudo install /tmp/wardhook /usr/local/bin/wardhook
```

### `go install`

```bash
go install github.com/chrisfelesoid/wardhook/cmd/wardhook@latest
```

## Quick Start

### 1. Register the hook in `.claude/settings.json`

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [{ "type": "command", "command": "wardhook" }]
      }
    ]
  }
}
```

### 2. Create `wardhook.yaml`

```yaml
version: 1
rules:
  - name: block-rm-recursive
    tool: Bash
    match:
      command: rm
      flags_all: [r, f]
      flag_aliases:
        r: [recursive]
        f: [force]
    except:
      glob:
        mode: all
        patterns: ["/tmp/**", "**/build/**", "**/node_modules/**"]
    action: deny

  - name: deny-sensitive-files
    tool: "*"
    match:
      glob:
        mode: any
        patterns: ["**/.env", "**/.env.*", "~/.ssh/**", "~/.aws/**"]
    action: deny
```

### 3. Validate

```bash
wardhook validate --config wardhook.yaml
# OK
```

### 4. Try a rule locally

Use `wardhook test` to see how a single command flows through your rules. It is a local debugging tool — exit code does not reflect the decision.

```bash
wardhook test --rule block-rm-recursive 'rm -fr ./important'
# config: wardhook.yaml
# tool:   Bash
# rules:  block-rm-recursive (1 of 5)
# input:  rm -fr ./important
#
# parsed commands (1):
#   [0] name=rm flags=[f,r] args=[./important] raw="rm -fr ./important"
#
# rule trace:
#   block-rm-recursive (tool=Bash, action=deny)
#     [0] MATCH -> deny
#
# final: deny
# reason: [wardhook] denied by rule "block-rm-recursive": rm -fr ./important
```

- `--rule NAME` may be repeated; without it, all rules in the config are evaluated.
- `--tool TOOL` defaults to `Bash`. Supported: `Bash`, `Read`, `Write`, `Edit`, `NotebookEdit`, `Glob`, `WebFetch`, `WebSearch`. `Grep` is not supported because it needs both a path and a pattern.
- When exactly one `--rule` is passed and its `tool` is a concrete name, that tool is used automatically; otherwise the default is `Bash`.

### 5. Use with Codex (OpenAI)

To run wardhook from Codex CLI's `pre-tool-use` hook, register the `codex` subcommand. Refer to Codex's official hook configuration documentation for the exact file format and key — the configuration shape is still evolving.

```bash
wardhook codex < codex-pre-tool-use.json
```

Codex emits the same Claude-vocabulary `tool_name` (`"Bash"`, `"Read"`, ...) and `tool_input` (`{"command": "..."}`, `{"file_path": "..."}`, ...) as Claude Code, so a single `wardhook.yaml` rule set applies to both.

> wardhook evaluates its own rules independently of Codex's `permission_mode`. Even when Codex runs in `bypassPermissions` mode, wardhook `deny` rules still block the call.

## Configuration

wardhook reads `wardhook.yaml` (override via `--config`).
The schema is strict — unknown keys are rejected at load time.

### Top-level

```yaml
version: 1                    # required, must be 1
defaults: { ... }             # optional, see below
rules: [ ... ]                # required, list of rules
```

### `defaults` block

| Field | Type | Default | Purpose |
| --- | --- | --- | --- |
| `cli_specs` | `map[string]CLISpec` | built-ins (bash, sh, docker, podman, kubectl, gcloud, nsenter) | Per-CLI parsing knowledge and recursion. User entries are additive. See `cli_specs` section. |
| `recursive_max_depth` | `int` | `3` | Maximum recursion count |

### `rules` entries

| Field | Type | Required | Purpose |
| --- | --- | --- | --- |
| `name` | `string` | yes | Rule identifier used in `reason` output |
| `tool` | `string` | yes | `Bash` / `Read` / `Write` / `Edit` / `Glob` / `Grep` / `WebFetch` / `WebSearch` / `NotebookEdit` / `*` |
| `match` | `MatchSpec` | yes | What this rule matches |
| `except` | `MatchSpec` | no | Sub-conditions that cancel the match |
| `action` | `string` | yes | `allow` / `deny` / `ask` |
| `reason` | `string` | no | Custom human-readable explanation |

### `MatchSpec`

| Field | Type | Purpose |
| --- | --- | --- |
| `command` | `string` | First word of a Bash command (e.g. `rm`). Empty matches any command. |
| `flags_all` | `[]string` | All listed flags must match (after alias normalization) |
| `flags_any` | `[]string` | At least one listed flag must match |
| `flag_aliases` | `map[string][]string` | Local alias table: `r: [recursive]` treats `--recursive` as `-r` |
| `flag_values` | `[]FlagValueMatch` | Match against captured flag values. Each entry is `{name, glob?, regex?}` (at least one of `glob`/`regex` required; both → AND). See "Flag value matching" below. |
| `glob` | `*GlobMatch` | Glob match against `Command.Args`. `{mode: any\|all, patterns: [...]}`. See "Glob matching". |
| `regex` | `*RegexMatch` | Regex match against `Command.Args`. `{mode: any\|all, patterns: [...]}`. AND'd with `glob` when both are present. See "Regex matching". |

### Aggregation

For multiple commands (from `;`, `&&`, `|`, `$()`, `bash -c` expansion), each is evaluated independently and the strictest result wins: `deny > ask > allow`.
The `reason` field reports the winning rule.

### Decision flow

```text
1. Read PreToolUse JSON from stdin.
2. Load wardhook.yaml.
   - missing       → allow (opt-in design)
   - parse error   → ask
3. Parse tool_input.
   - Bash    via mvdan.cc/sh; expand cli_specs wrappers.
   - others  via field map (file_path / url / pattern).
4. Evaluate each Command against each Rule:
   - tool match? (or "*")
   - match satisfied?
   - except not satisfied?
5. Aggregate decisions (deny > ask > allow).
6. Emit hookSpecificOutput JSON to stdout, exit 0.
```

## Glob matching

`MatchSpec.glob` and `FlagValueMatch.glob` declare a glob match with an explicit `mode` and a list of `patterns`:

```yaml
glob:
  mode: any | all
  patterns:
    - "<doublestar pattern>"
    - ...
```

### `mode: any`

The match succeeds if **at least one** input matches **at least one** pattern.
Example deny list: "block when any dangerous path appears".

```yaml
match:
  command: rm
  flags_all: [r, f]
  glob:
    mode: any
    patterns: ["/etc/**", "/usr/**", "/var/**"]
action: deny
```

`rm -rf /etc /tmp` → `/etc` hits `/etc/**` → **deny**

### `mode: all`

The match succeeds only if **every** input matches at least one pattern.
Example allow list (typically inside `except`): "allow only if every arg is in the safe zone".

```yaml
match:
  command: rm
  flags_all: [r, f]
except:
  glob:
    mode: all
    patterns: ["/tmp/**", "**/build/**"]
action: deny
```

`rm -rf /tmp/x` → all args under `/tmp` → except succeeds → **allow**
`rm -rf /tmp/x /etc/passwd` → `/etc/passwd` does NOT match → except fails → **deny**

### Inputs (string literals)

Inputs (either `Command.Args` for top-level `glob` or captured flag values for `FlagValueMatch.Glob`) are matched verbatim.
No `cwd` resolution, no `~` expansion, no path absolutization. Write your patterns to match the actual strings:

```yaml
patterns:
  - "/etc/**"           # absolute path
  - "**/.env"           # any depth
  - "~/.ssh/**"         # literal tilde (input must contain ~)
  - "./scratch/**"      # literal ./ prefix
```

`doublestar.Match` treats `/` as path separator and `**` as a multi-segment wildcard.

### Empty input case

| input set | `mode: any` | `mode: all` |
| --- | --- | --- |
| empty (e.g. no args, missing flag) | false | **false** (fail-closed) |
| at least one element | depends on match | depends on match |

For safety, `mode: all` treats **an empty input set as "not satisfied" (false)**. This prevents an absent flag or empty arg list from inadvertently passing an allow list.

## Regex matching

`regex` declares a Go RE2 regex match against `Command.Args` or flag values. Use this when patterns require character classes, quantifiers, anchors, or order independence that `glob` cannot express.
For example, catching `chmod a+wrx` regardless of the order of `r`/`w`/`x`.

```yaml
glob:                                # path-like patterns
  mode: any
  patterns: ["/etc/**", "/usr/**"]
regex:                               # complex / order-independent patterns
  mode: any
  patterns:
    - '^[0-7]?[0-9]?7[0-9]?7$'      # 777, 0777, 4777, ...
    - '^[augo]*\+[rwx]*w[rwx]*$'    # a+rwx, a+wrx, ugo+xwr, ...
```

### Combining `glob` and `regex`

When both are declared in the same `match` or `except` block, they are **AND'd**. All Commands must satisfy both rules.

```yaml
match:
  command: chmod
  regex: { mode: any, patterns: ['^[0-7]?777$'] }   # dangerous perm
  glob:  { mode: any, patterns: ["/etc/**"] }       # dangerous path
```

This catches `chmod 777 /etc/passwd` (both conditions hit) but allows `chmod 777 /tmp/cache` (path safe) and `chmod 644 /etc/hosts` (perm safe).

### Use cases

| Use case | Recommended |
| --- | --- |
| File paths (`/etc/**`, `**/.env`) | `glob` |
| Simple wildcards (`prod-*`) | `glob` |
| Order-independent permutations | `regex` (`^[augo]*\+[rwx]+$`) |
| Character classes (`[rwx]`, `\d`) | `regex` |
| Anchors (`^...$`) | `regex` |
| Both path-like and complex | `glob` + `regex` (AND) |

### Empty input case

| input set | `mode: any` | `mode: all` |
| --- | --- | --- |
| empty | false | **false** (fail-closed) |
| at least one element | depends | depends |

Same as `glob`: `mode: all` treats **an empty input set as "not satisfied" (false)**.

### Constraints

- Go RE2 syntax only — **no lookahead, lookbehind, or backreferences**
- Patterns are not auto-anchored — use `^...$` for full-string match
- Patterns are compiled at config load time; invalid syntax produces a load error

## Flag value matching

`flag_values` distinguishes between flag values — e.g. block `terraform -chdir=environments/prod apply` while allowing `terraform -chdir=environments/dev apply`.

### Declaration

```yaml
rules:
  - name: deny-terraform-prod
    tool: Bash
    match:
      command: terraform
      flag_values:
        - name: chdir
          glob:
            mode: any
            patterns:
              - "environments/prod"
              - "environments/prod/**"
    action: deny

  - name: deny-kubectl-prod
    tool: Bash
    match:
      command: kubectl
      flag_values:
        - name: n               # works for a 1-char short flag too
          glob:
            mode: any
            patterns: ["prod", "prod-*"]
    action: deny
```

Within a single entry, `glob` patterns are **OR**. Across multiple entries, they are **AND** (all must match).
If the same flag appears multiple times (e.g. `-var foo -var bar`), at least one of the captured values must match.

### Regex on captured flag values

`flag_values[].regex` lets you match captured values with Go RE2:

```yaml
- name: deny-kubectl-prod-versioned
  tool: Bash
  match:
    command: kubectl
    flag_values:
      - name: n
        regex:
          mode: any
          patterns: ['^prod(-\d+)?$']    # prod, prod-1, prod-42, ...
  action: deny
```

If both `glob` and `regex` are declared for a single `flag_values` entry,
they are AND'd:

```yaml
flag_values:
  - name: var
    glob:  { mode: all, patterns: ["*=*"] }                # k=v form
    regex: { mode: all, patterns: ['^[A-Z_]+=[^/]+$'] }    # uppercase key, no slash in value
```

### Supported syntax forms

wardhook recognizes a value when any of these appears:

| Form | Example |
| --- | --- |
| `=` form, double-dash | `kubectl --namespace=prod` |
| `=` form, single-dash long | `terraform -chdir=environments/prod` |
| `=` form, short | `cmd -n=prod` |
| Space form, double-dash long | `kubectl --namespace prod` |
| Space form, single-dash long | `terraform -chdir environments/prod` |
| Space form, single-char short | `kubectl -n prod` |
| Attached form, single-char short | `kubectl -nprod` |
| Bundled, value-taking char in middle | `cmd -vn prod` (v then n=prod) |

The `=` form is captured **unconditionally**.
For space, attached, and bundled forms, the flag must be declared in at least one rule's `flag_values[].name`.
This tells the parser to consume the next token as a value instead of treating it as a positional argument.

### Fail-closed semantics

When a declared value-taking flag has no value (end of command), the parent Command is degraded to `ask` — the same fail-closed behavior used for broken `cli_specs` wrappers.

| Trigger | Behavior |
| --- | --- |
| `terraform -chdir` (no value follows) | parent → `ask` |
| `kubectl -n` (no value follows) | parent → `ask` |
| `--chdir=` (empty value after `=`) | normal — captured as empty string. Rules that want to forbid empties can use `glob: {mode: any, patterns: [""]}` |

### Caveats

- `flag_values` declarations in **any rule** affect parsing across **all rules**: the parser builds a single value-taking set up front.
  This is necessary so that `terraform -chdir env/prod` consistently treats `env/prod` as a value rather than as a positional argument.
- doublestar's `**` matches across path separators. For non-path values (e.g. namespace names like `prod-*`) where there is no `/`,
  the pattern still works as a regular wildcard.
- Use canonical names in `flag_values[].name`. If you also declared `flag_aliases`, both the canonical name and its alt spellings
  are registered as value-taking so either form is parsed correctly.
- `glob.mode: all` evaluates as **false** when the input set is empty (fail-closed). This prevents `flag_values` allowlists from
  matching when the flag is absent.

## cli_specs: per-CLI parsing and recursion

`cli_specs` declares per-CLI knowledge that the parser uses to
(a) correctly partition flags from positional arguments, and
(b) extract embedded nested commands so rules can match against them.

wardhook ships built-in entries for the most common wrapper CLIs:

| CLI | value_taking_flags | recurse.flags | recurse.terminator | recurse.subcommands |
| --- | --- | --- | --- | --- |
| `bash`, `sh` | — | `[c]` | — | — |
| `docker`, `podman` | 80+ flags (`name`, `volume`, `v`, `env`, `e`, `p`, ...) | — | yes | `run: {skip: 1}`, `exec: {skip: 1}` |
| `kubectl` | 20+ flags (`namespace`, `n`, `context`, `container`, `c`, ...) | — | yes | `exec: {skip: 1}` |
| `gcloud` | 15+ flags (`project`, `region`, `zone`, ...) | `[command]` | yes | — |
| `nsenter` | 20+ flags (`target`, `t`, `mount`, `m`, ...) | — | yes | — |

If you write no `cli_specs` in your YAML, these built-ins apply automatically.
User declarations are additive and extend the built-in settings.

### Three recursion strategies

For each CLI, wardhook tries every declared strategy in parallel and re-parses every extracted nested command:

#### `recurse.flags: [c]`

The value of the named flag is the nested command. Used for `bash -c "rm -rf /"` and `gcloud --command="rm -rf /etc"`.
Short names (1 char) match `-c <value>`; long names (multi-char) match `--command=<value>` or `--command <value>`.

#### `recurse.terminator: true`

Words after the literal `--` token are joined with spaces and re-parsed. Used for `kubectl exec pod -- rm -rf /`,
`nsenter --target 1234 -- cmd`, and `docker exec ct -- cmd`.

#### `recurse.subcommands: {<verb>: {skip: N}}`

When the first argument matches `<verb>`, wardhook skips `N` further arguments and treats the rest as the nested command.
This is what catches `docker run [opts] IMAGE CMD ARGS`: after the `run` verb, the next argument (`IMAGE`) is skipped, and the remaining tokens form the nested command.

For this to work, **`value_taking_flags` must be accurate** for the CLI — otherwise a flag value gets misclassified as a positional and the IMAGE position shifts.
Built-in entries for docker/kubectl include the common value-taking flags. If you use an obscure flag, declare it in your user `cli_specs`:

```yaml
defaults:
  cli_specs:
    docker:
      value_taking_flags: [my-custom-flag]   # added to the built-in set
```

### Examples

```yaml
defaults:
  cli_specs:
    # Add a new CLI not covered by built-ins
    helm:
      value_taking_flags: [namespace, n, kubeconfig, context]
      recurse:
        terminator: true
    # Extend docker with a custom flag
    docker:
      value_taking_flags: [my-org-flag]

rules:
  - name: block-rm-rf
    tool: Bash
    match:
      command: rm
      flags_all: [r, f]
    action: deny
```

Catches:

- `rm -rf /etc` → deny
- `bash -c "rm -rf /etc"` → deny (via `bash.recurse.flags: [c]`)
- `docker run -it --name x ubuntu rm -rf /etc` → deny (via `docker.recurse.subcommands.run`)
- `docker exec my-ct rm -rf /etc` → deny (via `docker.recurse.subcommands.exec`)
- `kubectl exec pod -c sidecar rm -rf /etc` → deny (via `kubectl.recurse.subcommands.exec`)
- `gcloud compute ssh vm -- rm -rf /etc` → deny (via `gcloud.recurse.terminator`)
- `nsenter --target 1234 -- rm -rf /etc` → deny (via `nsenter.recurse.terminator`)

### Recursive depth

```yaml
defaults:
  recursive_max_depth: 3   # default
```

Limits how deep recursion can go (e.g. `bash -c "bash -c rm"` is depth 2).
Commands that hit the limit are marked as InspectionFailed and degraded to `ask`.

### Fail-closed semantics

When wardhook cannot inspect the nested command (subcommand verb found but skip target missing, broken inner Bash quoting, depth exceeded, missing flag value), the parent Command is degraded to `ask`.

| Trigger | Behavior |
| --- | --- |
| `docker run` (no image, truncated) | parent → `ask` |
| `bash -c "echo 'broken"` (broken quoting) | parent → `ask` |
| `bash -c` (missing value) | parent → `ask` |
| Depth > `recursive_max_depth` | parent → `ask` |


## License

[MIT](LICENSE) © 2026 chrisfelesoid
