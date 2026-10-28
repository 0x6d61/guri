# ファイルシステム操作

`(guri filesystem)`は、Linux、macOS、Windowsで使用するGuriの拡張ライブラリです。
#101で追加しました。
R7RS-small標準の`(scheme file)`と分離し、標準ライブラリの束縛を変更しません。
[SRFI 170](https://srfi.schemers.org/srfi-170/srfi-170.html)の名前と契約を参考にしていますが、`(srfi 170)`は提供しません。
REPLでも明示的にimportして使用します。

## 共通の規則

パスはStringで指定します。
相対パスはプロセスの現在の作業ディレクトリを基準に解決します。
実行するスクリプトの配置場所やライブラリの検索経路を基準にしません。
パスの区切り、使用できる名前、大文字小文字の区別はOSとファイルシステムの規則に従います。
シェル展開、環境変数の展開、`~`の展開は行いません。

ファイルシステム操作の失敗は、`error-object?`と`file-error?`を満たす例外として送出し、`guard`で捕捉できます。
メッセージには手続き名と原因、irritantsには渡した引数を含めます。
型と引数数の誤りは、`error-object?`を満たし、`file-error?`を満たさない例外です。
OSから返された名前をSchemeのUnicode Stringに変換できない場合も、置換や黙った除外をせず、ファイルエラーとします。

返すListとStringは呼び出しごとに新しく作成します。
返した値を変更しても、ファイルシステムや別の呼び出しの結果は変わりません。
複数の操作をまとめたトランザクションや、他のプロセスによる変更からの保護は提供しません。
調査後に対象が変わる場合があるため、判定手続きの成功は後続の操作の成功を保証しません。

## ディレクトリの列挙

`(directory-files path [dotfiles?])`は1または2引数を取り、直下の名前をStringのListで返します。
`dotfiles?`はBooleanで、省略時は`#f`です。
`#f`では`.`で始まる名前を除外し、`#t`では含めます。
`.`と`..`は常に除外します。
Windowsのhidden属性を判定するオプションではありません。
返す名前に`path`の接頭辞は含めず、列挙順は保証しません。
空のディレクトリでは空Listを返します。
ディレクトリへのリンクは追跡します。
対象の不在、通常ファイルの指定、権限不足はファイルエラーです。
ファイルシステムは変更せず、列挙に使用した資源は内部で閉じます。

## ディレクトリの作成と削除

`(create-directory path)`は1引数を取り、1階層だけ作成して未規定値を返します。
親ディレクトリは作成しません。
同名のファイル、ディレクトリ、リンクの存在や、親の不在はファイルエラーです。
Unixでは作成モード`#o775`をumaskで制限します。
WindowsではOSのアクセス制御に従い、POSIXのモードと同じACLを保証しません。

`(delete-directory path)`は1引数を取り、空の実ディレクトリを削除して未規定値を返します。
通常ファイル、リンク、非空ディレクトリ、存在しない名前はファイルエラーです。
再帰削除やリンク先の削除は行いません。

## 名前の変更

`(rename-file old new)`は2つのStringを取り、名前を変更して未規定値を返します。
リンクを指定するとリンク自体を移動します。
他のファイルシステムへの移動をコピーと削除へ置き換えません。
既存の宛先がある場合、ディレクトリと非ディレクトリの間の型の競合を事前検査してファイルエラーにします。
検査後の他プロセスによる変更は防げません。
型が一致する宛先の置換や原子性の制約はOSの名前変更操作に従い、3 OSで同じ置換保証を提供しません。
失敗時の回避策として宛先を先に削除しません。
[Goのos.Renameの契約](https://pkg.go.dev/os#Rename)も参照してください。
宛先を保護したい場合、事前の存在確認だけでは、他プロセスとの競合による置換を防げません。
不在の移動元、型の競合、権限不足などはファイルエラーです。

## ファイルの判定とサイズ

`(file-directory? path [follow?])`と`(file-regular? path [follow?])`は1または2引数を取り、Booleanを返します。
`follow?`はBooleanで、省略時は`#t`です。
`#t`ではリンク先を調べ、`#f`では最後のパス要素のリンクを追跡しません。
途中のディレクトリ要素のリンクはOSの規則に従って解決します。
前者はディレクトリ、後者は通常ファイルである場合に`#t`を返します。
デバイスやパイプは通常ファイルに含めません。
存在しないパスや、追跡時にリンク先がない場合は`#f`です。
権限不足など、OSが不在以外と判定した失敗はファイルエラーです。
ファイルシステムは変更しません。

`(file-size path)`は1引数を取り、通常ファイルのバイト数を正確なIntegerで返します。
リンクは追跡します。
ファイルシステムは変更しません。
不在、通常ファイル以外、権限不足はファイルエラーです。

## ファイルのコピー

`(copy-file source destination)`は2つのStringを取り、通常ファイルの内容をバイナリでコピーして未規定値を返します。
元ファイルへのリンクは追跡します。
宛先は排他的に新規作成し、既存の通常ファイル、ディレクトリ、リンクを上書きしません。
リンク先が存在しない宛先リンクもエラーです。
コピーしたファイルの内容は元ファイルから独立します。
所有者、ACL、時刻、元ファイルのモードは複製しません。
Unixでは宛先の作成モード`#o666`をumaskで制限します。
元ファイルが通常ファイルでない場合、読み書きやcloseの失敗、既存の宛先はファイルエラーです。
読み書きのために開いた資源は、成功と失敗の両方で閉じます。
コピー中の失敗や割り込みでは、作成済みの部分ファイルが残る場合があります。
ロールバックや失敗時の自動削除は保証しません。

## 使用例

次の例は、引数に既存の空ディレクトリを渡して実行します。
日本語名のファイルを作成し、コピー、名前変更、列挙、削除を行います。
最後に作成したファイルとディレクトリを削除します。

```scheme
(import (scheme base) (scheme file) (scheme process-context)
        (scheme write) (guri filesystem))
(define root (cadr (command-line)))
(define work (string-append root "/work"))
(define source (string-append work "/元.txt"))
(define copy (string-append work "/copy.txt"))
(define renamed (string-append work "/renamed.txt"))
(create-directory work)
(call-with-output-file source (lambda (port) (write-string "hello" port)))
(copy-file source copy)
(rename-file copy renamed)
(write (list (file-directory? work)
             (file-regular? renamed)
             (file-size renamed)
             (length (directory-files work))))
(newline) ; (#t #t 5 2)
(delete-file source)
(delete-file renamed)
(delete-directory work)
```

失敗を捕捉する例です。
引数に既存のディレクトリを渡すと、通常ファイルではないため`#t`を表示します。

```scheme
(import (scheme base) (scheme process-context)
        (scheme write) (guri filesystem))
(write (guard (e ((file-error? e) #t) (else (raise e)))
         (file-size (cadr (command-line)))
         #f))
(newline) ; #t
```
