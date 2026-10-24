# datumとファイルの入出力

このページの13手続きは内部の`core.NewCore`環境で利用できます。
[ポート基盤の38束縛](https://github.com/0x6d61/guri/wiki/Ports)と合わせて使用します。
公開CLIと公開Go APIの切替は後続作業です。
標準ライブラリのimportは内部Runtimeで利用でき、[提供範囲](https://github.com/0x6d61/guri/wiki/Libraries)を区別します。
基準は[R7RS-small §6.13](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html)です。
readはscheme read、write系とdisplayはscheme write、ファイル操作はscheme fileへの割り当てとして台帳で追跡しています。

## readとポートの位置

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(read [port])` | 0または1、テキスト入力ポート | 次のdatumまたはEOF。datumの末尾まで入力を消費する |

ポートを省略するとcurrent-input-portを使用します。
readは新しい可変データを生成し、同じdatum内のラベル参照は同一オブジェクトを共有します。
ラベルの有効範囲は1回のreadです。
前のreadのラベルを次のreadから参照することはできません。

readとread-char、peek-char、read-lineは同じポートの位置を使用します。
字句判定で先読みした文字を別のReader内へ取り去らず、後続の文字入力へ残します。
括弧や引用符で完結したdatumの後に、ストリーム全体のEOFが来るまで待つことはありません。
トークンの終わりを判定するために区切り文字が必要な場合は、その文字またはEOFを待ちます。

#!fold-case／#!no-fold-caseの状態はポートごとに保持します。
datumが始まる前のEOFはEOF値を返し、ポートを開いた状態に保ちます。
不完全なdatum、不正なラベルやUTF-8はread-error?を満たす例外を送出します。
エラーメッセージには入力の名前・行・列を含め、未処理の例外には呼び出し元の位置も保持します。
不正な型・方向、バイナリや閉じたポートへのreadは通常のerror-objectを送出します。

```scheme
(define p (open-input-string "(1 . 2) x"))
(list (read p) (read-char p) (read p) (eof-object? (read p)))
; => ((1 . 2) #\space x #t)
```

```scheme
(define p (open-input-string "#0=(1 . #0#)"))
(define a (read p))
(eq? a (cdr a)) ; => #t
```

```scheme
(define p (open-input-string "#!fold-case FOO Bar #!no-fold-case BAZ"))
(list (read p) (read p) (read p)) ; => (foo bar BAZ)
```

```scheme
(guard (e ((read-error? e) #t) (else #f))
  (read (open-input-string "(unfinished"))) ; => #t
```

## writeとdisplay

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(write obj [port])` | 1または2、任意の値とテキスト出力 | 外部表現を書く。循環がある場合にラベルを使う。結果は未規定 |
| `(write-shared obj [port])` | 1または2、任意の値とテキスト出力 | 共有するPair／Vectorもラベルで表す。結果は未規定 |
| `(write-simple obj [port])` | 1または2、任意の値とテキスト出力 | ラベルを使わず書く。循環では停止しない場合がある。結果は未規定 |
| `(display obj [port])` | 1または2、任意の値とテキスト出力 | Stringは内容、Characterは文字、Symbolは名前をそのまま書く。結果は未規定 |

ポートを省略するとcurrent-output-portを使用します。
これらは入力の値を変更せず、出力ポートを変更します。
表示のためにデータをコピーしたり、共有関係を変更したりしません。
displayはPairやVectorの内側でも上記の文字・文字列・Symbolの規則を使い、循環はラベルで表して停止します。
Recordなど外部表現のない値には内部Writerの不透明な表記を使い、その表記からreadで復元できるとは扱いません。

不正な型・方向、バイナリや閉じたポートへの出力、短い書き込みや出力先の失敗はerror-objectを送出します。
write-simpleの循環処理を含め、Writerは実行ごとのGo contextによるキャンセルを確認します。
これはSchemeの例外ハンドラで解除する制限ではなく、Goへのキャンセルエラーとして返します。
全手続きの引数数の不一致は、既存VMと同じく自動raiseへ未接続です。

```scheme
(define p (open-output-string))
(parameterize ((current-output-port p))
  (display "猫")
  (write '(1 2))
  (newline))
(get-output-string p) ; => "猫(1 2)\n"
```

```scheme
(define p (open-output-string))
(define a (list 1))
(write-shared (list a a) p)
(get-output-string p) ; => "(#0=(1) #0#)"
```

```scheme
(define p (open-output-string))
(define a (list "x"))
(set-cdr! a a)
(display a p)
(get-output-string p) ; => "#0=(x . #0#)"
```

## ファイルの規則（D07-F）

2026-10-24にユーザーが以下の方針を承認しました。

- ファイル名はOSのパスとして扱い、相対パスは実行時の作業ディレクトリを基準にします。
- テキストの内容はUTF-8で読み書きし、不正な符号化を置き換えずエラーにします。
- 出力は新規作成し、同名のファイルがある場合は開いた時点で長さを0にして書き直します。

R7RSは既存の出力ファイルの扱いを未規定としており、この上書き動作はGuriの選択です。
新規ファイルの権限は0666を基にOSのumaskを適用します。
ファイル探索、loadとライブラリの検索規則はこの合意に含めません。

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(open-input-file filename)` | 1、String | 既存ファイルの新しいテキスト入力ポート |
| `(open-binary-input-file filename)` | 1、String | 既存ファイルの新しいバイナリ入力ポート |
| `(open-output-file filename)` | 1、String | ファイルを作成／切り詰めた新しいテキスト出力ポート |
| `(open-binary-output-file filename)` | 1、String | ファイルを作成／切り詰めた新しいバイナリ出力ポート |

各呼び出しは独立したファイル位置と開閉状態を持ちます。
ポートの代入は同じオブジェクトを参照するため、別名はその状態を共有します。
これらのファイルはGuriが所有し、ポートのcloseでOS資源も閉じます。
ファイルを開けない場合はfile-error?を満たす例外を送出し、不正な引数の型は通常のerror-objectにします。
パスのStringは変更しません。

## ファイルを使う手続きの呼び出し

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(call-with-input-file filename proc)` | 2、Stringと1引数の手続き | テキスト入力ポートをprocへ渡し、通常復帰時に閉じて全ての値を返す |
| `(call-with-output-file filename proc)` | 2、Stringと1引数の手続き | テキスト出力ポートをprocへ渡し、通常復帰時に閉じて全ての値を返す |
| `(with-input-from-file filename thunk)` | 2、Stringと0引数の手続き | current-input-portを新しいテキスト入力に束縛し、thunkの全ての値を返す |
| `(with-output-to-file filename thunk)` | 2、Stringと0引数の手続き | current-output-portを新しいテキスト出力に束縛し、thunkの全ての値を返す |

with-*-fileは正常復帰時にファイルを閉じ、以前のcurrent portへ戻します。
継続や例外による脱出時は以前の束縛へ戻し、再開可能なファイルを自動では閉じません。
捕捉した継続へ再入すると、同じポートとその時点の入力位置・開閉状態を使用します。
閉じたファイルを開き直したり、入力位置を巻き戻したりしません。
call-with-*-fileでも、通常復帰と継続による脱出に対するcloseの規則はcall-with-portと同じです。

以下の例は、それぞれ独立した一時ディレクトリで検証します。

```scheme
(call-with-output-file "example.scm"
  (lambda (p) (write '(1 2 3) p)))
(call-with-input-file "example.scm" read) ; => (1 2 3)
```

```scheme
(with-output-to-file "example.txt" (lambda () (display "42")))
(with-input-from-file "example.txt" (lambda () (read))) ; => 42
```

```scheme
(define out (open-binary-output-file "example.bin"))
(write-bytevector #u8(0 255) out)
(close-port out)
(define in (open-binary-input-file "example.bin"))
(define b (read-bytevector 10 in))
(close-port in)
b ; => #u8(0 255)
```
