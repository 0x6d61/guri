# golang.org/x/text

Unicodeの完全なcase foldingを実装するため、Go公式の補助パッケージを使用します。
標準ライブラリの単純な小文字化では、ßからssへの変換などを満たせません。

- モジュール：golang.org/x/text
- 固定版：v0.43.0
- 出典：https://go.googlesource.com/text/+/refs/tags/v0.43.0/
- 使用箇所：internal/readerの#!fold-case
- ライセンス：[LICENSE](LICENSE)。追加の特許権許諾は[PATENTS](PATENTS)。いずれも原文を保持。
- 完全性：go.modとgo.sumで版とチェックサムを固定。

コード本体はGo Modulesで取得します。
初回ビルド前に`go mod download`を実行し、その後のunitテストはネットワークを無効にして実行できます。
Unicodeのデータ版はGoコンパイラに対応して選択され、Go 1.26では15.0.0です。
Go更新時は、標準ライブラリunicodeとcasesのデータ版の一致も確認します。
