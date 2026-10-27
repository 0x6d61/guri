# CLIとインストール

## ビルドと起動

Go 1.26以降でリポジトリのルートからビルドします。

```sh
go build -o build/guri ./cmd/guri
./build/guri --version       # guri 0.0.3
./build/guri examples/counter.scm
./build/guri                 # REPL
```

| 指定 | 動作 |
|---|---|
| 引数なし | 継続した対話環境でREPLを実行する |
| `script.scm args...` | ファイルをプログラムとして実行する。最後の値は自動表示しない |
| `- args...` | 標準入力全体をプログラムとして実行する |
| `-I DIR`、`-IDIR` | 外部ライブラリの検索先を指定順に追加する。複数回指定可能 |
| `--` | 処理系オプションの解析を終了する |
| `--help` | 使用方法を表示して終了する |
| `--version` | バージョンを表示して終了する |

最初のスクリプト指定以降は、`--flag`などもスクリプトの引数になります。
`command-line`の先頭はスクリプト名、`-`実行では`"-"`、REPLでは起動コマンド名です。
ファイル・`-`プログラムは最初に必要なライブラリをimportします。
`-`は標準入力をソースとして読み切るため、実行時の同じ入力ポートはEOFになります。

```scheme
(import (scheme base) (scheme write) (scheme process-context))
(display "Hello, Scheme!")
(newline)
```

## REPLと貼り付け

REPLは16標準ライブラリを初期環境で提供し、定義を次の入力へ保持します。
式が返す各値を改行付きで表示し、未規定値とゼロ個の値は表示しません。
通常のReader・評価エラーは標準エラーへ表示してから次の入力へ進みます。
`exit`や`emergency-exit`ではREPL自体を終了します。

標準入力と標準出力が端末の場合、行編集と `guri> ` プロンプトを使用します。
bracketed paste対応端末では貼り付け範囲を認識します。
複数行を貼り付けた後にEnterを押すとまとめて実行し、貼り付け途中へプロンプトを挿入しません。
端末モードは終了時に復元します。
入力または出力をリダイレクトした場合は、プロンプトや端末制御列を出しません。

```sh
printf '(+ 1 2)\n' | ./build/guri  # 3
```

## 入出力と終了

起動時の標準入力・出力・エラーをSchemeのcurrent portsへ接続します。
OSから受け取ったストリームは借用し、Schemeの`close-port`で元のOSストリームを閉じません。
Schemeのポート自体は閉じた状態になるため、以後の使用はエラーになります。

| 状態 | 終了コード |
|---|---|
| 正常終了 | 0 |
| 通常のプログラム・入出力エラー | 1 |
| 不正なCLIオプション・必須引数不足 | 2 |
| `exit` / `emergency-exit` | 省略または`#t`は0、`#f`は1、正確な整数は下位8ビット、それ以外は1 |

`exit`は必要な`dynamic-wind`のafterを実行し、開いている出力をflushします。
`emergency-exit`はafterとフロントエンドの終了時flushを省略します。
明示した終了コードはflush診断によって変更しません。
エラーには可能な範囲でソース名・行・列と原因を含めます。
[プロセスと終了](https://github.com/0x6d61/guri/wiki/System-Interface)、[Port](https://github.com/0x6d61/guri/wiki/Ports)も参照してください。

## インストールと検索先

```sh
make install PREFIX="$HOME/.local"
# パッケージ作成時はDESTDIRで配置先だけを移せる
make install PREFIX=/usr/local DESTDIR=/tmp/guri-package
```

インストール先は`<prefix>/bin/guri`です。
`<prefix>/share/guri/site/lib`と`<prefix>/share/guri/0.0.3/lib`を作り、その絶対パスをリンカでバイナリへ保持します。
`DESTDIR`はパッケージの一時配置先であり、埋め込む検索パスには含めません。
移設した場合は、再インストールまたは`-I`/`GURI_LOAD_PATH`で検索先を指定してください。

16標準ライブラリはGo実装としてバイナリへ組み込まれます。
外部`.sld`の検索順は次のとおりです。

1. `-I`で明示したディレクトリを指定順に検索する。
2. `GURI_LOAD_PATH`をOSのパス区切りで分割した順に検索する。
3. インストール時の`<prefix>/share/guri/site/lib`を検索する。
4. インストール時の`<prefix>/share/guri/0.0.3/lib`を検索する。

直接`go build`した開発バイナリでは、未設定のインストール先を探索しません。
カレントディレクトリの`lib/`、スクリプトの隣、`examples/`を暗黙の検索先にしません。
スクリプトとライブラリは別々の任意の場所へ置けます。
[ライブラリ仕様](https://github.com/0x6d61/guri/wiki/Libraries)に名前とパスの対応、標準名の保護、includeの基準を記載しています。

## 対応状況

公開CLIは17件とChibi由来62件の回帰ケースで成功しています。
これをR7RS-small全体の準拠率として扱いません。
#95でLinux、macOS、WindowsのポートReadyとOS別CIを検証しました。
確認した環境と結果は[結合監査](https://github.com/0x6d61/guri/wiki/R7RS-Audit)に記録します。
Guri 0.0.3はR7RS-smallへの準拠を宣言し、`r7rs` featureを公開します。
数値表現と実装依存事項、確認したOSとアーキテクチャは[結合監査](https://github.com/0x6d61/guri/wiki/R7RS-Audit)を参照してください。
旧ルートGo APIは削除し、新しい公開Go APIは提供していません。
