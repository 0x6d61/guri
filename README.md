# Guri

Guriは**R7RS-smallに準拠したScheme処理系**です。
Goで実装したバイトコードVMで実行し、単体のCLIでアプリケーションを作ることを主用途とします。
現在のバージョンは **0.0.3** です。
全16標準ライブラリを提供し、`features`と`cond-expand`で`r7rs`を公開します。

## ビルドと実行

Go 1.26以降を使用します。

```sh
go build -o build/guri ./cmd/guri
./build/guri examples/counter.scm
./build/guri                 # REPL
./build/guri --version
```

[counter.scm](examples/counter.scm)は `1`、`2` の順に出力します。
ファイルのプログラムは最初に必要なライブラリをimportします。
ファイル実行では最終式の値を自動表示しません。

```scheme
(import (scheme base) (scheme write))
(define (make-adder n) (lambda (x) (+ n x)))
(define add10 (make-adder 10))
(display (add10 5))
(newline) ; 15
```

REPLは16標準ライブラリの束縛を初期状態で利用でき、式の値を表示します。
端末では `guri> ` を表示します。
bracketed paste対応端末では、複数行を貼り付けた後にEnterを押すとまとめて実行します。
貼り付けの途中にプロンプトを挿入しません。
パイプからのREPL入力にはプロンプトを出しません。

```sh
printf '(+ 1 2)\n' | ./build/guri
printf '(import (scheme base) (scheme write)) (display 42)\n' | ./build/guri -
./build/guri -I /path/to/libraries /anywhere/program.scm arg1 arg2
```

`-` は標準入力全体をプログラムとして読み込みます。
`command-line`はスクリプト名と残りの引数を返し、REPLでは起動コマンド名を返します。
CLIの使用方法の誤りは終了コード2、通常の実行エラーは1、成功は0です。
REPLの通常エラーは表示後に次の入力を受け付けます。
詳細は[CLIとインストール](docs/wiki/CLI.md)を参照してください。

## インストールとライブラリ

```sh
make build
make install PREFIX="$HOME/.local"
```

`make install`はバイナリとライブラリ配置用ディレクトリを作ります。
16標準ライブラリはGoの実装としてバイナリへ組み込まれており、別ファイルのコピーは不要です。
外部ライブラリの検索順は `-I`、`GURI_LOAD_PATH`、`<prefix>/share/guri/site/lib`、`<prefix>/share/guri/0.0.3/lib` です。
開発用の直接ビルドではインストール先を仮定しません。
cwd、スクリプトの隣、`examples/`、リポジトリの `lib/` は暗黙に探索しません。
詳細は[ライブラリ仕様](docs/wiki/Libraries.md)を参照してください。

## 仕様と対応状況

- [言語仕様](docs/wiki/Language-Specification.md)と[構文](docs/wiki/Special-Forms.md)
- [全16標準ライブラリの557公開束縛](docs/wiki/Functions.md)（構文・補助キーワードを含む）
- [VM設計](docs/wiki/Bytecode-VM.md)、[要求台帳と未決事項](docs/wiki/R7RS-Conformance.md)
- [テストの実行方法](docs/r7rs/TESTING.md)と[監査結果](docs/wiki/R7RS-Audit.md)
- [GitHub Wiki](https://github.com/0x6d61/guri/wiki)、[Wiki公開手順](docs/WIKI.md)

旧Guriの独自構文、Map/Keyword、Go Slice型Vector、Bytes、およびルートのGo APIは削除しました。
SchemeのPair、固定長Vector、Bytevector、可変Unicode Stringへ移行しています。
旧テスト56件の移行・廃止理由は[対応表](docs/r7rs/legacy-tests.csv)で追跡します。
新しい組み込み用Go APIは今回提供しません。

公開CLIの回帰ケースは17件とChibi由来62件が成功しています。
Chibi原本の監査では、実行した1,214判定がすべて成功し、保留は0件です。
Linux、macOS、WindowsのポートReadyとOS別CIは#95で検証しました。
数値は任意精度整数、正確な有理数、binary64と正確／不正確な複素数を提供します。
任意の精度指定 `s/f/d/l` は採用しておらず、原本の該当10フォームを成功に数えず記録しています。
数値表現と実装依存事項は[仕様書](docs/wiki/Language-Specification.md)、動作確認環境と検証の範囲は[監査結果](docs/wiki/R7RS-Audit.md)に記載しています。

準拠を表すfeatureはプログラムから確認できます。

```scheme
(import (scheme base) (scheme write))
(write (and (pair? (memq 'r7rs (features)))
            (cond-expand (r7rs #t) (else #f))))
(newline) ; #t
```

## 開発

```sh
go test ./...
go vet ./...
```

開発フローは[AGENTS.md](AGENTS.md)、外部コードの条件は[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)を参照してください。
