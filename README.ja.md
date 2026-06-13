# wardhook

[English](README.md) | 日本語

AI コーディングエージェント向けの、設定可能な PreToolUse フックフィルタです。
`settings.json` のパーミッションでは捕捉できない破壊的な操作をブロックしつつ、安全な操作は許可します。


[![License](https://img.shields.io/github/license/chrisfelesoid/wardhook)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/chrisfelesoid/wardhook)](go.mod)
[![Release](https://img.shields.io/github/v/release/chrisfelesoid/wardhook)](https://github.com/chrisfelesoid/wardhook/releases)

## 特徴

- **フラグの順序に依存しないマッチング**: `flags_all: [r, f]` は `-rf` / `-fr` / `-r -f` などの順不同でマッチングします。
- **シェルメタ文字の分解**: `;`、`&&`、`||`、`|`、`$()`、バッククォートをサブコマンドに分解し、それぞれを評価します。
- **再帰的なラッパー展開**: `bash -c "..."` や `gcloud compute ssh ... -- ...` を再パースし、内側のコマンドにもルールを適用します。
- **ツール横断のパスグロブ**: `tool: "*"` のルール 1 つで `Read` / `Write` / `Edit` / `Bash` のすべてに対して `.env` 系ファイルをブロックできます。
- **`except` 句**: `/tmp/**` への書き込みは許可しつつ、それ以外を拒否するといった指定が可能です。
- **Fail-closed**: パースエラーや未知の構造は `ask` にフォールバックし、`allow` にはしません。

## インストール

```bash
# 例: Linux amd64
curl -L -o /tmp/wh.tar.gz \
  https://github.com/chrisfelesoid/wardhook/releases/latest/download/wardhook_<version>_linux_amd64.tar.gz
tar -xzf /tmp/wh.tar.gz -C /tmp/
sudo install /tmp/wardhook /usr/local/bin/wardhook
```

### `go install`

```bash
go install github.com/chrisfelesoid/wardhook/cmd/wardhook@latest
```

## クイックスタート

### 1. `.claude/settings.json` にフックを登録

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

### 2. `wardhook.yaml` を作成

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

### 3. 設定確認

```bash
wardhook validate --config wardhook.yaml
# OK
```

### 4. ルールをローカルで試す

`wardhook test` で 1 つのコマンドがルール群にどう流れるかを確認できます。

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

- `--rule NAME` は複数指定可。未指定なら config の全ルールを評価します。
- `--tool TOOL` のデフォルトは `Bash`。サポート対象は `Bash` / `Read` / `Write` / `Edit` / `NotebookEdit` / `Glob` / `WebFetch` / `WebSearch`。`Grep` は path と pattern の 2 値が必要なため非対応です。
- `--rule` が 1 つだけ指定され、そのルールの tool が具体名なら自動的にその tool を使います。それ以外は `Bash` がデフォルトです。

### 5. Codex (OpenAI) で使う

Codex CLI の `pre-tool-use` フックから wardhook を呼ぶには `codex` サブコマンドを登録します。設定ファイルの場所やキー名は Codex の公式ドキュメントを参照してください（現状フォーマットが流動的なため）。

```bash
wardhook codex < codex-pre-tool-use.json
```

Codex は Claude Code と同じ `tool_name` (`"Bash"`, `"Read"`, ...) と `tool_input` (`{"command": "..."}`, `{"file_path": "..."}`, ...) を発火するため、1 つの `wardhook.yaml` を両方の CLI に共用できます。

> wardhook は Codex の `permission_mode` に依存せず独自にルールを評価します。Codex が `bypassPermissions` モードで動作していても、wardhook の `deny` ルールはそのまま作動します。

### 6. Cursor で使う

Cursor の `preToolUse` フックから wardhook を呼ぶには `.cursor/hooks.json` に `wardhook cursor` を登録します。

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      { "command": "wardhook cursor", "failClosed": true }
    ]
  }
}
```

`failClosed: true` の指定を推奨します。wardhook 自体が異常終了した場合でも、allow へフォールバックさせずブロックさせるためです。

Cursor はシェル実行を `Shell` ツール名で発火しますが、wardhook は内部で `Bash` (Claude 語彙) に正規化するため、既存の `tool: Bash` ルールがそのまま適用されます。Cursor 固有のツール (`Delete`, `Task`, `MCP:*`) は元の名前で `tool:` に書くか、`tool: "*"` のクロスツールルールでまとめてマッチできます。

> wardhook は Cursor 側の permission 状態に依存せず独自にルールを評価します。Cursor 側の承認フローに関わらず、wardhook の `deny` ルールは常に作動します。

> wardhook は parse / config エラー時に `ask` をフェイルクローズドのフォールバックとして出力します。現状の Cursor は `permission: "ask"` を受け付けますが、将来仕様変更で拒否されるようになった場合、それらのエッジケースは Cursor 側のエラー挙動に委ねられます。

## 設定

wardhook は `wardhook.yaml` を読み込みます (`--config` で上書き可能)。
スキーマは厳格で、未知のキーはロード時に拒否されます。

### トップレベル

```yaml
version: 1                    # 必須、必ず 1
defaults: { ... }             # 任意、後述
rules: [ ... ]                # 必須、ルールのリスト
```

### `defaults` ブロック

| フィールド | 型 | デフォルト | 用途 |
| --- | --- | --- | --- |
| `cli_specs` | `map[string]CLISpec` | 組み込み (bash, sh, docker, podman, kubectl, gcloud, nsenter) | CLI 単位のパース知識および再帰展開。ユーザーエントリは加算的に追加されます。`cli_specs` セクション参照。 |
| `recursive_max_depth` | `int` | `3` | 再帰の最大数 |

### `rules` エントリ

| フィールド | 型 | 必須 | 用途 |
| --- | --- | --- | --- |
| `name` | `string` | yes | `reason` 出力で使われるルール識別子 |
| `tool` | `string` | yes | `Bash` / `Read` / `Write` / `Edit` / `Glob` / `Grep` / `WebFetch` / `WebSearch` / `NotebookEdit` / `*` |
| `match` | `MatchSpec` | yes | このルールがマッチする条件 |
| `except` | `MatchSpec` | no | マッチを打ち消す副条件 |
| `action` | `string` | yes | `allow` / `deny` / `ask` |
| `reason` | `string` | no | カスタムの人間可読な説明 |

### `MatchSpec`

| フィールド | 型 | 用途 |
| --- | --- | --- |
| `command` | `string` | Bash コマンドの先頭の単語 (例: `rm`)。空文字列は任意のコマンドにマッチ。 |
| `flags_all` | `[]string` | 列挙されたフラグがすべて一致 (エイリアス正規化後) |
| `flags_any` | `[]string` | 列挙されたフラグのうち少なくとも 1 つが一致 |
| `flag_aliases` | `map[string][]string` | ローカルなエイリアステーブル: `r: [recursive]` で `--recursive` を `-r` と等価に扱う |
| `flag_values` | `[]FlagValueMatch` | 捕捉したフラグの値に対するマッチ。各エントリは `{name, glob?, regex?}` (`glob`/`regex` のいずれかが必須、両方指定時は AND)。下記「フラグ値のマッチング」を参照。 |
| `glob` | `*GlobMatch` | `Command.Args` に対するglobマッチ。`{mode: any\|all, patterns: [...]}`。「globマッチ」を参照。 |
| `regex` | `*RegexMatch` | `Command.Args` に対する正規表現マッチ。`{mode: any\|all, patterns: [...]}`。`glob` と併用すると AND。「正規表現マッチ」を参照。 |

### 集約

複数のコマンド (`;`、`&&`、`|`、`$()`、`bash -c` 展開から得られるもの) はそれぞれ独立に評価され、最も厳しい結果で評価されます:`deny > ask > allow`
`reason` フィールドには判定結果のルールが出力されます。

### 判定フロー

```text
1. PreToolUse の JSON を stdin から読む。
2. wardhook.yaml をロード。
   - ファイルなし → allow (オプトイン設計)
   - パースエラー → ask
3. tool_input をパース。
   - Bash    : mvdan.cc/sh でパースし、cli_specs のラッパーを展開。
   - その他  : フィールドマップ (file_path / url / pattern) でパース。
4. 各 Command を各 Rule に対して評価:
   - tool が一致するか (または "*")
   - match が成立するか
   - except が成立しないか
5. 判定を集約 (deny > ask > allow)。
6. hookSpecificOutput JSON を stdout に出力し、exit 0。
```

## globマッチ

`MatchSpec.glob` と `FlagValueMatch.glob` は、明示的な `mode` と `patterns` のリストでglobマッチを定義します:

```yaml
glob:
  mode: any | all
  patterns:
    - "<doublestar パターン>"
    - ...
```

### `mode: any`

入力のうち **少なくとも 1 つ** が、パターンのうち **少なくとも 1 つ** にマッチすれば成立します。
例として拒否リスト: 「危険なパスが 1 つでも含まれていたらブロック」

```yaml
match:
  command: rm
  flags_all: [r, f]
  glob:
    mode: any
    patterns: ["/etc/**", "/usr/**", "/var/**"]
action: deny
```

`rm -rf /etc /tmp` → `/etc` が `/etc/**` にヒット → **deny**

### `mode: all`

**すべての** 入力がいずれかのパターンにマッチした場合にのみ成立します。
例として許可リスト(通常は `except` の中): 「すべての引数が安全領域内であれば許可」

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

`rm -rf /tmp/x` → 全引数が `/tmp` 配下 → except 成立 → **allow**
`rm -rf /tmp/x /etc/passwd` → `/etc/passwd` が非マッチ → except 不成立 → **deny**

### 入力(文字列リテラル)

入力 (トップレベル `glob` では `Command.Args`、`FlagValueMatch.Glob` では捕捉済みのフラグ値) は文字どおり照合されます。
`cwd` 解決も、`~` 展開も、絶対パス化も行いません。実際の文字列に合うパターンを書いてください:

```yaml
patterns:
  - "/etc/**"           # 絶対パス
  - "**/.env"           # 任意の深さ
  - "~/.ssh/**"         # リテラルなチルダ (入力に ~ が含まれている必要あり)
  - "./scratch/**"      # リテラルな ./ プレフィックス
```

`doublestar.Match` は `/` をパス区切りとして、`**` を複数セグメントのワイルドカードとして扱います。

### 入力が空の場合

| 入力集合 | `mode: any` | `mode: all` |
| --- | --- | --- |
| 空 (引数なし、フラグ欠落など) | false | **false** (fail-closed) |
| 1 つ以上の要素 | マッチ次第 | マッチ次第 |

`all` モードでは、安全性を考慮し**「入力が空の場合も条件未達成（false）」**と扱います。フラグ欠落や引数なしによって、意図せず許可リストを通過してしまうのを防ぐためです。

## 正規表現マッチ

`regex` は `Command.Args` またはフラグ値に対する正規表現マッチを定義します。文字クラス、量化子、アンカー、順序非依存な表現など、`glob` では表現できないパターンに使います。
例えば、`r`/`w`/`x` の順序によらず `chmod a+wrx` を捕捉する用途などです。

```yaml
glob:                                # パス系のパターン
  mode: any
  patterns: ["/etc/**", "/usr/**"]
regex:                               # 複雑/順序非依存なパターン
  mode: any
  patterns:
    - '^[0-7]?[0-9]?7[0-9]?7$'      # 777, 0777, 4777, ...
    - '^[augo]*\+[rwx]*w[rwx]*$'    # a+rwx, a+wrx, ugo+xwr, ...
```

### `glob` と `regex` の併用

同じ `match` または `except` ブロックに両方が宣言された場合、これらは **AND** で結合されます。全 Command が両方のルールを満たす必要があります。

```yaml
match:
  command: chmod
  regex: { mode: any, patterns: ['^[0-7]?777$'] }   # 危険な権限
  glob:  { mode: any, patterns: ["/etc/**"] }       # 危険なパス
```

これは `chmod 777 /etc/passwd` を捕捉する一方で (両条件ヒット)、`chmod 777 /tmp/cache` (パスが安全) や `chmod 644 /etc/hosts` (権限が安全) は許可します。

### ユースケース

| ユースケース | 推奨 |
| --- | --- |
| ファイルパス (`/etc/**`, `**/.env`) | `glob` |
| 単純なワイルドカード (`prod-*`) | `glob` |
| 順序非依存な順列 | `regex` (`^[augo]*\+[rwx]+$`) |
| 文字クラス (`[rwx]`, `\d`) | `regex` |
| アンカー (`^...$`) | `regex` |
| パス系かつ複雑 | `glob` + `regex` (AND) |

### 入力が空の場合

| 入力集合 | `mode: any` | `mode: all` |
| --- | --- | --- |
| 空 | false | **false** (fail-closed) |
| 1 つ以上の要素 | マッチ次第 | マッチ次第 |

`glob` と同様、`mode: all` も**「入力が空の場合も条件未達成（false）」**と扱います。

### 制約

- Go RE2 構文のみ — **先読み、後読み、後方参照は不可**
- パターンは自動的にアンカーされません — 全文一致には `^...$` を使ってください
- パターンは設定ロード時にコンパイルされ、無効な構文はロードエラーになります

## フラグ値のマッチング

`flag_values` はフラグの値で挙動を分けるためのものです。例えば
`terraform -chdir=environments/prod apply` はブロックしつつ
`terraform -chdir=environments/dev apply` は通す、といった用途です。

### 宣言

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
        - name: n               # 1 文字の短いフラグでも動作
          glob:
            mode: any
            patterns: ["prod", "prod-*"]
    action: deny
```

単一エントリ内では `glob` のパターン同士は **OR**。複数エントリ間では**AND** (すべて満たす必要あり)。
同一フラグが複数回現れる場合 (例: `-var foo -var bar`)、捕捉値のうち少なくとも 1 つがマッチすれば成立します。

### 捕捉したフラグ値に対する正規表現

`flag_values[].regex` で、捕捉値に対する Go RE2 マッチが可能です:

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

単一の `flag_values` エントリに `glob` と `regex` の両方を宣言した場合、
それらは AND で結合されます:

```yaml
flag_values:
  - name: var
    glob:  { mode: all, patterns: ["*=*"] }                # k=v 形式
    regex: { mode: all, patterns: ['^[A-Z_]+=[^/]+$'] }    # 大文字キー、値にスラッシュなし
```

### サポートされる構文形式

wardhook は以下のいずれかの形を値として認識します:

| 形式 | 例 |
| --- | --- |
| `=` 形式、ダブルダッシュ | `kubectl --namespace=prod` |
| `=` 形式、シングルダッシュ long | `terraform -chdir=environments/prod` |
| `=` 形式、short | `cmd -n=prod` |
| スペース形式、ダブルダッシュ long | `kubectl --namespace prod` |
| スペース形式、シングルダッシュ long | `terraform -chdir environments/prod` |
| スペース形式、1 文字 short | `kubectl -n prod` |
| 連結形式、1 文字 short | `kubectl -nprod` |
| バンドル、途中に値取得 char | `cmd -vn prod` (v の次に n=prod) |

`=` 形式は **無条件** に値として捕捉されます。
スペース形式、連結形式、バンドル形式では、いずれかのルールの `flag_values[].name` にそのフラグが定義されている必要があります。
これによりパーサは次のトークンを位置引数ではなく値として消費すべきだと判断できます。

### Fail-closed セマンティクス

値取得フラグとして宣言されたフラグに値が後続しない (コマンド末尾) 場合、
親 Command は `ask` に降格されます。これは `cli_specs` ラッパーの破損時と
同じ fail-closed の挙動です。

| トリガー | 挙動 |
| --- | --- |
| `terraform -chdir` (値なし) | 親 → `ask` |
| `kubectl -n` (値なし) | 親 → `ask` |
| `--chdir=` (`=` の後に空) | 通常動作 — 空文字列として捕捉。空を禁じたい場合は `glob: {mode: any, patterns: [""]}` を使う |

### 注意事項

- **どこかのルール** に書かれた `flag_values` 宣言は **すべてのルール** の
  パースに影響します。パーサは値取得フラグの集合を 1 度だけ事前構築するためです。
  これは `terraform -chdir env/prod` の `env/prod` が常に値として扱われ、
  位置引数と混同されないようにするために必要です。
- doublestar の `**` はパス区切りをまたいでマッチします。`/` を含まない
  非パス値 (例: `prod-*` のような名前空間) でも、通常のワイルドカードとして機能します。
- `flag_values[].name` には canonical 名を使ってください。`flag_aliases` を
  併せて宣言している場合は、canonical 名と代替表記の両方が値取得フラグとして
  登録されるので、どちらの表記でも正しくパースされます。
- `glob.mode: all` は入力集合が空のとき **false** と評価されます (fail-closed)。
  これにより、フラグが存在しないときに `flag_values` の許可リストが成立してしまうことを防ぎます。

## cli_specs: CLI 単位のパースと再帰展開

`cli_specs` は CLI 単位の知識を宣言するもので、パーサはこれを使って
(a) フラグと位置引数を正しく分離し、
(b) 埋め込まれた内側のコマンドを抽出して
ルール照合の対象にします。

wardhook は代表的なラッパー CLI に対して以下の組み込みエントリを提供します:

| CLI | value_taking_flags | recurse.flags | recurse.terminator | recurse.subcommands |
| --- | --- | --- | --- | --- |
| `bash`, `sh` | — | `[c]` | — | — |
| `docker`, `podman` | 80+ flags (`name`, `volume`, `v`, `env`, `e`, `p`, ...) | — | yes | `run: {skip: 1}`, `exec: {skip: 1}` |
| `kubectl` | 20+ flags (`namespace`, `n`, `context`, `container`, `c`, ...) | — | yes | `exec: {skip: 1}` |
| `gcloud` | 15+ flags (`project`, `region`, `zone`, ...) | `[command]` | yes | — |
| `nsenter` | 20+ flags (`target`, `t`, `mount`, `m`, ...) | — | yes | — |

YAML に `cli_specs` を書かない場合、これらの組み込みが自動的に適用されます。
ユーザーの定義は追加で、組み込みの設定を拡張します。

### 3 種類の再帰戦略

各 CLI について、wardhook は宣言されたすべての戦略を並列に試し、
抽出されたネストコマンドをそれぞれ再パースします:

#### `recurse.flags: [c]`

指定フラグの値がネストコマンドです。`bash -c "rm -rf /"` や
`gcloud --command="rm -rf /etc"` などで使われます。
短い名前 (1 文字) は `-c <value>` に、長い名前 (複数文字) は `--command=<value>` または `--command <value>` にマッチします。

#### `recurse.terminator: true`

リテラル `--` トークン以降の単語をスペースで連結して再パースします。
`kubectl exec pod -- rm -rf /`、`nsenter --target 1234 -- cmd`、
`docker exec ct -- cmd` などで使われます。

#### `recurse.subcommands: {<verb>: {skip: N}}`

最初の引数が `<verb>` にマッチしたら、wardhook はさらに `N` 個の引数をスキップし、残りをネストコマンドとして扱います。
これが `docker run [opts] IMAGE CMD ARGS` を捕捉する仕組みです: `run` の後ろの次の引数 (`IMAGE`) がスキップされ、残りのトークンがネストコマンドになります。

これが正しく動作するには、その CLI の **`value_taking_flags` が正確である必要があります**。
そうでないとフラグ値が位置引数と誤判定され、IMAGE の位置がずれます。
docker/kubectl の組み込みエントリは一般的な値取得フラグを含んでいます。
マイナーなフラグを使う場合は、ユーザー側の `cli_specs` で宣言してください:

```yaml
defaults:
  cli_specs:
    docker:
      value_taking_flags: [my-custom-flag]   # 組み込み集合に追加される
```

### 例

```yaml
defaults:
  cli_specs:
    # 組み込みにない CLI を追加
    helm:
      value_taking_flags: [namespace, n, kubeconfig, context]
      recurse:
        terminator: true
    # docker をカスタムフラグで拡張
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

これで以下が捕捉されます:

- `rm -rf /etc` → deny
- `bash -c "rm -rf /etc"` → deny (`bash.recurse.flags: [c]` 経由)
- `docker run -it --name x ubuntu rm -rf /etc` → deny (`docker.recurse.subcommands.run` 経由)
- `docker exec my-ct rm -rf /etc` → deny (`docker.recurse.subcommands.exec` 経由)
- `kubectl exec pod -c sidecar rm -rf /etc` → deny (`kubectl.recurse.subcommands.exec` 経由)
- `gcloud compute ssh vm -- rm -rf /etc` → deny (`gcloud.recurse.terminator` 経由)
- `nsenter --target 1234 -- rm -rf /etc` → deny (`nsenter.recurse.terminator` 経由)

### 再帰の深さ

```yaml
defaults:
  recursive_max_depth: 3   # デフォルト
```

再帰がどこまで潜れるかを制限します (例: `bash -c "bash -c rm"` は深さ 2)。
上限に達したコマンドは InspectionFailed としてマークされ、`ask` に降格されます。

### Fail-closed セマンティクス

wardhook が内側コマンドを検査できない場合 (サブコマンド verb は見つかったがスキップ対象が足りない、内側 Bash のクォートが壊れている、深さ超過、フラグ値の欠落)、親 Command は `ask` に降格されます。

| トリガー | 挙動 |
| --- | --- |
| `docker run` (イメージなし、途中で切れている) | 親 → `ask` |
| `bash -c "echo 'broken"` (クォート破損) | 親 → `ask` |
| `bash -c` (値欠落) | 親 → `ask` |
| 深さが `recursive_max_depth` を超過 | 親 → `ask` |


## ライセンス

[MIT](LICENSE) © 2026 chrisfelesoid
