# 外部プロセスとパイプ接続

`(guri process)`は、Linux、macOS、Windowsで外部プロセスを同期実行するGuriの拡張ライブラリです。
`run-process`と`run-pipeline`を提供し、REPLでも明示的にimportします。
バイナリに含まれるため、追加のライブラリファイルは不要です。
R7RS-smallの標準手続きには含めません。

## 手続き

| 構文 | 引数数 | 返り値（3値） |
|---|---|---|
| `(run-process argv [options])` | 1または2 | 終了コード、標準出力Bytevector、標準エラーBytevector |
| `(run-pipeline commands [options])` | 1または2 | 各段の終了コードのList、最終段の標準出力Bytevector、各段の標準エラーBytevectorのList |

`argv`は空でないStringのListです。
先頭を実行ファイル、残りを引数として渡します。
実行ファイル名は空文字にできませんが、残りの引数には空文字を渡せます。
`commands`は`argv`の空でないListで、記述した順に各段の標準出力と次の段の標準入力をOSのパイプで接続します。
1段だけのパイプラインも実行できます。

すべての直接の子プロセスを待機して回収し、入出力の処理を終えてから返します。
シェルを暗黙に挟まないため、引数の空白、引用符、`$`、`|`、`*`は文字どおり渡します。
実行ファイルのPATH検索には親の環境を使い、カレントディレクトリからの暗黙の実行はGoの`os/exec`の規則に従って拒否します。
特定のファイルを実行する場合は絶対パス、または`./program`などの明示的なパスを使います。
相対パスの実行ファイルは`directory`で指定したディレクトリを基準に実行します。
Windowsでの引数の引用はGoの`os/exec`が行います。
独自のコマンドライン解析を行う実行ファイルには、その実行ファイル固有の制約があります。

## オプション

`options`はSymbolをキーとするドット対の関連リストです。
省略時は空の関連リストと同じです。
パイプラインでは全段に同じ作業ディレクトリと環境変数を適用し、`input`は最初の段だけに渡します。

| キー | 型 | 省略時 | 意味 |
|---|---|---|---|
| `input` | Bytevector | 空 | 子の標準入力へ渡すバイト列 |
| `directory` | String | 親の作業ディレクトリ | 子の作業ディレクトリ |
| `environment` | StringからStringへのドット対の関連リスト | 親の環境を継承 | 親の環境に指定した変数を上書きして渡す |

たとえば`'((input . #u8(65 10)) (directory . ".") (environment . (("MODE" . "test"))))`と指定します。
`environment`に空の関連リストを渡しても、親の環境を継承します。
親の作業ディレクトリと環境変数は変更しません。
`directory`の空文字は省略時と同じです。
`environment`の`PATH`は子に渡す値の上書きであり、この呼び出し自体の実行ファイルの検索には使いません。
Windowsの環境変数名は大文字と小文字を区別しません。

## 終了状態とエラー

終了コードは正確な非負整数です。
0を成功とし、非0終了も例外にせず通常の結果として返します。
Unixでシグナルによって終了した段は`#f`を返します。
Windowsの終了コードは符号なし32ビットの整数で返し、Unixのシグナル番号へ変換しません。
パイプラインの終了コードと標準エラーは、最初の段から順に並びます。
後段が成功しても、前段の非0終了や`#f`を成功へ置き換えません。

次の場合は、`guard`などで捕捉できる通常のSchemeエラーを通知します。

- 引数数、型、関連リストの項目が不正。
- `argv`や`commands`が空、Listが不適切または循環している。
- 未知のオプション、重複するオプション名や環境変数名がある。
- 引数やパス、環境変数にNULがある、環境変数名が空または`=`を含む。
- 実行ファイルが見つからない、実行権限がない、作業ディレクトリが存在しないなどの起動失敗。
- 起動後の入出力や子プロセスの回収が失敗する。

全段の引数を検証してから起動します。
起動途中の失敗では、起動済みの直接の子プロセスを終了して回収し、所有するパイプを閉じてからエラーを返します。
途中までの出力を成功結果として返しません。
子が入力を読み切らずに終了した場合の入力側のbroken pipeは通常の終了として扱い、終了コードを返します。
子プロセス同士のbroken pipeによる終了は、該当段の終了状態として返します。

内部Runtimeへ渡したcontextがキャンセルされた場合も、直接の子プロセスと入出力処理を終了してから中断を返します。
この中断はScheme例外へ変換しません。
CLI自体のOSシグナルによる強制終了や、子孫プロセス全体の終了は、この回収保証の対象外です。

## データと資源の所有

引数のList、String、Bytevectorを変更しません。
渡す入力バイト列は起動前にコピーし、返すBytevectorとListは呼び出しごとに新しく作ります。
返り値の変更が引数や別の呼び出しの返り値へ反映されることはありません。
親の標準入力を暗黙に読み取らず、親の標準ポートを閉じません。
子の出力は捕捉して返し、親の標準出力や標準エラーへ自動では表示しません。

標準入力への送信、最終出力、各段の標準エラーの読み取りを並行して進めます。
VMでのSchemeコードの評価はシングルスレッドのままです。
中間段の標準出力はOSのパイプを通し、全量をメモリへ保存しません。
最終出力と各段の標準エラーはメモリに保持するため、大きさに応じてメモリを消費します。
任意長の出力を一定量のメモリで処理する公開ストリームAPIと、非同期プロセスオブジェクトは提供しません。
子孫プロセスが出力のパイプを保持した場合は、EOFまたはcontextのキャンセルまで読み取りを待ちます。

## Guri同士を接続する例

次のプログラムを`pipeline.scm`へ保存し、`guri pipeline.scm /absolute/path/to/guri`で実行します。
同じプログラムを`--child`付きで2回起動し、`hello`をパイプへ流します。
標準エラーは段ごとに捕捉され、標準出力と混ざらず返ります。

```scheme
(import (scheme base) (scheme read) (scheme write)
        (scheme process-context) (guri process))

(define args (cdr (command-line)))
(if (and (pair? args) (string=? (car args) "--child"))
    (begin
      (display "child diagnostic\n" (current-error-port))
      (let loop ((line (read-line)))
        (unless (eof-object? line)
          (display line)
          (newline)
          (loop (read-line)))))
    (let ((child (list (car args) (car (command-line)) "--child")))
      (call-with-values
        (lambda ()
          (run-pipeline (list child child)
                        (list (cons 'input (string->utf8 "hello\n")))))
        (lambda (statuses output diagnostics)
          (write statuses)
          (newline)
          (display (utf8->string output))
          (for-each (lambda (bytes) (display (utf8->string bytes)))
                    diagnostics)))))
```

実行結果は次のとおりです。

```text
(0 0)
hello
child diagnostic
child diagnostic
```

同じ例をリポジトリの`examples/process-pipeline.scm`に収録しています。

## Gitリポジトリの状態を調べる例

`examples/git-status.scm`は、`(guri filesystem)`でディレクトリを探索し、見つけたGitリポジトリで`git status --short`を実行します。
GitがPATHにある環境で、次のように実行します。

```sh
guri examples/git-status.scm /path/to/projects
```

各リポジトリのパスと変更状況を表示します。
Gitのworktreeの`.git`ファイルも認識し、ディレクトリのシンボリックリンクへは再帰しません。
リポジトリを見つけたらその内部の探索は止めるため、入れ子のリポジトリやsubmoduleの一覧化は行いません。
探索元のパスやGitが出力するテキストは、有効なUnicode文字列とUTF-8であることを前提とします。
Gitの終了状態が非0または`#f`ならエラーで終了します。
このGitを使う例は、外部コマンドに依存しないunitテストとは分けて検証します。
