# リポジトリガイドライン

AIエージェントおよび開発者向けのガイドラインです。
このリポジトリでコードを変更する際は、以下の規約に従ってください。

## プロジェクト構成とモジュール構造

ディレクトリAとBを比較し、B側の重複ファイルを削除するGo製CLIツールです。集合モード・パス一致モード（`--match-path`）・自己重複検出モード（`--self`）の3モードがあります。

- `cmd/sfd/main.go`: CLIエントリーポイント（`index`, `plan`, `apply`）
- `internal/app/`: ユースケースとコマンドレベルのオーケストレーション
- `internal/domain/`: コアモデルと一致判定ルール
- `internal/infra/`: ファイルシステム走査、JSONL I/O、ハッシュ計算、パス安全性
- `internal/*/*_test.go`: 単体テストおよび結合テスト
- `DESIGN.md`, `ARCHITECTURE.md`: 機能設計と実装設計

新しいコードは、明確な境界変更が必要でない限り、既存の `internal/{app,domain,infra}` のレイヤ構造に従って配置してください。

## ビルド・テスト・開発コマンド

詳細は [DEVELOPER.ja.md](DEVELOPER.ja.md) を参照してください。主なコマンドは以下の通りです。

- `go build ./cmd/sfd` — CLIバイナリのビルド
- `go run ./cmd/sfd --help` — ローカル実行
- `go test ./...` — 全テストの実行
- `gofmt -w $(go list -f '{{.Dir}}' ./...)` — 全Goファイルのフォーマット

## コーディングスタイルと命名規則

- 標準Goスタイルに従い、常に `gofmt` を実行する
- パッケージ名は小文字かつ簡潔（`app`, `domain`, `infra`）
- ファイル名は役割で命名（例: `index_usecase.go`, `jsonl_reader.go`）
- エラーは明示的にラップする（`fmt.Errorf("context: %w", err)`）
- CLIの動作は決定論的かつ安全優先（`dry-run` 既定、`--execute` 明示）

## テストガイドライン

- Go標準の `testing` パッケージを使用する
- テスト名は `TestXxx` 形式とし、テスト対象パッケージの近くに配置する
- 以下の動作変更には必ずテストを追加/修正する:
  - パス安全性（`EnsureWithinRoot`）
  - index/updateの動作
  - エンドツーエンドの `index -> plan -> apply` フロー

## ドキュメントの言語方針

ドキュメントファイルは2言語で存在します。英語版が正 (authoritative) です。

| 英語版 (正) | 日本語版 (参考) |
|---|---|
| `README.md` | `README.ja.md` |
| `DESIGN.md` | `DESIGN.ja.md` |
| `ARCHITECTURE.md` | `ARCHITECTURE.ja.md` |
| `DEVELOPER.md` | `DEVELOPER.ja.md` |
| `EXISTING_TOOLS.md` | `EXISTING_TOOLS.ja.md` |
| `AGENTS.md` | `AGENTS.ja.md` |
| `CHANGELOG.md` | `CHANGELOG.ja.md` |

規則:
- ドキュメントを更新するときは、**英語版と日本語版の両方**を更新する
- 英語版のみを変更した場合 (緊急修正など) は、日本語版に未同期である旨を記載するか、同じコミットで更新する
- 日本語版は英語版の翻訳であり、片方にしか存在しない内容を作らない

## コミット・プルリクエストガイドライン

- コミットメッセージは短い命令形（英語）で記述する:
- **1コミット = 1目的。** 無関係な変更を1つのコミットに混在させない。例えば、新機能の追加と別機能のドキュメント更新は別コミットにする。
  - `Add architecture design for Go-based MVP`
  - `Finalize design decisions for A/B checksum workflow`
- PRには以下を含める:
  - 目的とスコープ
  - 主要な設計・動作変更
  - テスト結果（`go test ./...`）
  - リスク事項（ファイル削除、パス検証、後方互換性）
