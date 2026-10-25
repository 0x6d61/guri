# 外部由来のコードとライセンス

Guri本体のライセンスは[MIT](LICENSE)です。
次の外部コード、テストと派生物には、元のライセンス条件も適用されます。

## Chibi SchemeのR7RSテスト

- 出典：[ashinn/chibi-scheme](https://github.com/ashinn/chibi-scheme)
- リビジョン：`b2087f3a320a56f79ede5c546fd45fdde582c4ed`
- 原本：`tests/r7rs-tests.scm`
- 著作権表示：Copyright (c) 2009-2021 Alex Shinn。原本内の寄稿者の記載も保持。
- ライセンス：BSD 3-Clause。[元のCOPYING全文](testdata/r7rs/upstream/chibi/COPYING)
- 対象：[保存した原本](testdata/r7rs/upstream/chibi/r7rs-tests.scm)と[testdata/r7rs/chibi/](testdata/r7rs/chibi/)の派生テスト、および[マクロ監査の回帰テスト](internal/core/audit_macro_test.go)。

原本は改変せず保存し、派生ケースには出典と変更内容、ライセンス全文を記載しています。
ソースの再配布では元の著作権表示、条件と免責文を保持してください。
対象のコードをバイナリ形式で再配布する場合は、元のCOPYINGの条件に従って付属資料にも表示を保持してください。
著者の名前を、許可なくGuriの推奨や公認を示す宣伝に使用しません。

## golang.org/x/text

ReaderのUnicode case foldingに`golang.org/x/text/cases` v0.43.0を使用します。
標準ライブラリの小文字化では不足する完全なcase foldingを提供する依存です。

- 著作権表示：Copyright 2009 The Go Authors.
- ライセンス：BSD 3-Clause。[LICENSE全文](docs/licenses/golang.org-x-text/LICENSE)
- 追加の特許権許諾：[PATENTS](docs/licenses/golang.org-x-text/PATENTS)
- 出典：https://go.googlesource.com/text/+/refs/tags/v0.43.0/

ソースやバイナリを配布する際は、元のライセンス条件に従い著作権表示、条件と免責文を保持してください。

## golang.org/x/term、golang.org/x/sys

端末REPLの行編集、bracketed paste、端末モードの復元に`golang.org/x/term` v0.47.0を使用します。
標準ライブラリには行編集と貼り付け範囲の認識がないため採用しました。
OSごとの端末操作で`golang.org/x/sys` v0.49.0を間接的に使用します。

- 著作権表示：Copyright 2009 The Go Authors.
- ライセンス：BSD 3-Clause。[termのLICENSE](docs/licenses/golang.org-x-term/LICENSE)、[sysのLICENSE](docs/licenses/golang.org-x-sys/LICENSE)
- 追加の特許権許諾：[termのPATENTS](docs/licenses/golang.org-x-term/PATENTS)、[sysのPATENTS](docs/licenses/golang.org-x-sys/PATENTS)
- 出典：[term v0.47.0](https://go.googlesource.com/term/+/refs/tags/v0.47.0/)、[sys v0.49.0](https://go.googlesource.com/sys/+/refs/tags/v0.49.0/)

バイナリを配布する際も、元のライセンス条件に従って著作権表示、条件と免責文を付属資料に保持してください。
