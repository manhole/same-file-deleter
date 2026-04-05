# リポジトリガイドライン

AIエージェントおよび開発者向けのガイドラインです。
このリポジトリでコードを変更する際は、以下の規約に従ってください。

## プロジェクト構成とモジュール構造

ディレクトリAとBを比較し、B側の重複ファイルを削除するGo製CLIツールです。A/B比較・パス一致（`--match-path`）・自己重複検出（`--self`）の3モードがあります。

- `cmd/sfd/main.go`: CLIエントリーポイント（`index`, `plan`, `apply`）
- `internal/app/`: ユースケースとコマンドレベルのオーケストレーション
- `internal/domain/`: コアモデルと一致判定ルール
- `internal/infra/`: ファイルシステム走査、JSONL I/O、ハッシュ計算、パス安全性
- `internal/*/*_test.go`: 単体テストおよび結合テスト
- `DESIGN.md`, `ARCHITECTURE.md`: 機能設計と実装設計

新しいコードは、明確な境界変更が必要でない限り、既存の `internal/{app,domain,infra}` のレイヤ構造に従って配置してください。

## ビルド・テスト・開発コマンド

詳細は [DEVELOPER.md](DEVELOPER.md) を参照してください。主なコマンドは以下の通りです。

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

## コミット・プルリクエストガイドライン

- コミットメッセージは短い命令形（英語）で記述する:
  - `Add architecture design for Go-based MVP`
  - `Finalize design decisions for A/B checksum workflow`
- PRには以下を含める:
  - 目的とスコープ
  - 主要な設計・動作変更
  - テスト結果（`go test ./...`）
  - リスク事項（ファイル削除、パス検証、後方互換性）
