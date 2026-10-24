# テキスト・バイナリポート

このページの38束縛は内部の`core.NewCore`環境で利用できます。
公開CLIと公開Go APIの切替は後続作業です。
標準ライブラリのimportは内部Runtimeで利用でき、[提供範囲](https://github.com/0x6d61/guri/wiki/Libraries)を区別します。
datumを扱うread、write、write-shared、write-simple、displayと、ファイル操作の13手続きも内部VMへ接続済みです。
構文・共有・エラーと使用例は[datumとファイルの入出力](https://github.com/0x6d61/guri/wiki/Datum-File-IO)を参照してください。
基準は[R7RS-small §6.13](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html)です。

## 型、共有と資源管理

ポートはテキストまたはバイナリの入力または出力ストリームです。
現在の内部アダプタでは入出力方向とテキスト／バイナリをそれぞれ区別し、双方向ポートは生成しません。
テキストストリームの符号化はUTF-8で、文字位置はUnicodeスカラー値を単位とします。
不正なUTF-8や途中で切れた文字はエラーにします。
代入や引数への受け渡しでは同じポートを参照し、入力位置と開閉状態を共有します。
EOFは入力の終端であり、ポートを閉じません。

Guriが開いた資源はGuriが管理し、closeで元の資源も閉じます。
Go側で開いて渡した資源は借用します。
借用ポートのcloseではポートを閉じて必要なflushを行いますが、借用元のGo資源のCloseを呼びません。
内部VMの初期current portsは、Goプロセスの標準入力・標準出力・標準エラーを借用したテキストポートです。
各Engineの開閉状態は独立し、元のOSストリームはプロセス側で共有します。
組み込み用の公開APIの詳細は将来の拡張で扱います。

以下では`[arg]`を省略可能な引数として表記します。
指定しない入力ポートはcurrent-input-port、出力ポートはcurrent-output-portを使用します。
`k`、`start`と`end`は非負の正確な整数です。
範囲はstartを含みendを含まず、`0 <= start <= end <= length`を満たす必要があります。
startの既定値は0、endの既定値は対象の長さです。
不正な型・方向・範囲、閉じたポートへのI/O、符号化、ストリームのエラーはerror-objectを非継続例外として送出し、guardで捕捉できます。
引数数の不一致はVMの引数数エラーであり、自動送出にはまだ接続していません。
read-error?とfile-error?は、接続済みのreadによる解析失敗とファイルを開く失敗を識別します。

## 述語とEOF

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(port? obj)` | 1、任意 | ポートなら#t。変更なし |
| `(input-port? obj)` | 1、任意 | 入力ポートなら#t。閉じた後も#t。変更なし |
| `(output-port? obj)` | 1、任意 | 出力ポートなら#t。閉じた後も#t。変更なし |
| `(textual-port? obj)` | 1、任意 | テキストポートなら#t。変更なし |
| `(binary-port? obj)` | 1、任意 | バイナリポートなら#t。変更なし |
| `(input-port-open? port)` | 1、ポート | 開いた入力ポートなら#t。変更なし |
| `(output-port-open? port)` | 1、ポート | 開いた出力ポートなら#t。変更なし |
| `(eof-object)` | 0 | EOF値。変更なし |
| `(eof-object? obj)` | 1、任意 | EOF値なら#t。変更なし |

## メモリ上のポート

| 構文 | 引数数・型 | 返り値、コピーと変更 |
|---|---|---|
| `(open-input-string str)` | 1、String | 内容をコピーした新しいテキスト入力ポート。strの変更を反映しない |
| `(open-output-string)` | 0 | 新しいテキスト出力ポート |
| `(get-output-string port)` | 1、open-output-stringの出力 | これまでの出力をコピーした新しい可変String。ポートは変更しない |
| `(open-input-bytevector bv)` | 1、Bytevector | 内容をコピーした新しいバイナリ入力ポート |
| `(open-output-bytevector)` | 0 | 新しいバイナリ出力ポート |
| `(get-output-bytevector port)` | 1、open-output-bytevectorの出力 | これまでの出力をコピーした新しい可変Bytevector。ポートは変更しない |

異なる種類の出力ポートからget-output-string／get-output-bytevectorを呼ぶとエラーです。
出力メモリポートを閉じた後でも、それまでの内容を取得できます。
返されたStringやBytevectorの変更はポートの内容に影響しません。

```scheme
(define p (open-input-string "λ猫"))
(list (peek-char p) (read-char p) (read-char p)
      (eof-object? (read-char p)) (input-port-open? p))
; => (#\λ #\λ #\猫 #t #t)
```

```scheme
(define p (open-output-bytevector))
(write-u8 255 p)
(write-bytevector #u8(1 2 3) p 1 3)
(close-port p)
(get-output-bytevector p) ; => #u8(255 2 3)
```

## current ports

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(current-input-port)` | 0 | 現在の入力ポート |
| `(current-output-port)` | 0 | 現在の出力ポート |
| `(current-error-port)` | 0 | 現在のエラー出力ポート |

これらはparameterizeで一時的に束縛できるパラメータです。
入力には入力ポート、出力とエラー出力には出力ポートを指定し、テキストとバイナリのどちらも指定できます。
個々のI/O手続きは要求するポートの種類を確認します。
parameterizeを出ると以前の束縛へ戻り、捕捉した継続へ再入すると捕捉時の束縛へ戻ります。
束縛の変更はポートを閉じたり、新しいポートを複製したりしません。

```scheme
(define p (open-output-string))
(parameterize ((current-output-port p))
  (write-string "λ")
  (newline))
(get-output-string p) ; => "λ\n"
```

## closeとcall-with-port

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(close-port port)` | 1、ポート | 閉じる。出力はflushする。結果は未規定 |
| `(close-input-port port)` | 1、入力ポート | 入力を閉じる。結果は未規定 |
| `(close-output-port port)` | 1、出力ポート | 出力をflushして閉じる。結果は未規定 |
| `(call-with-port port proc)` | 2、ポートと1引数を受け取る手続き | procに同じportを渡し、通常復帰時に閉じて全ての返り値を返す |

閉じたポートを再度closeしても何も行いません。
flushが失敗した場合も所有資源のcloseを試み、両方の原因を保持してエラーを返します。
失敗したcloseを繰り返して同じ資源を二度解放しません。

call-with-portのprocから継続や例外で脱出した場合は、後から再開できるため自動では閉じません。
捕捉した継続からprocへ戻り、通常復帰した時点で閉じます。
一度閉じたポートは継続への再入でも開き直しません。

```scheme
(define p (open-output-string))
(call-with-port p
  (lambda (out) (write-string "ok" out) (values 20 22)))
; => 20 22（2個の返り値）
```

```scheme
(define p (open-input-string "x"))
(close-port p)
(guard (e (else (error-object? e))) (read-char p)) ; => #t
```

## 入力

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(read-char [port])` | 0または1、テキスト入力 | 次のCharacterまたはEOF。1文字進める |
| `(peek-char [port])` | 0または1、テキスト入力 | 次のCharacterまたはEOF。論理的な入力位置を進めない |
| `(char-ready? [port])` | 0または1、テキスト入力 | 次のread-charが待たずに返せれば#t。EOFでも#t |
| `(read-line [port])` | 0または1、テキスト入力 | 改行を含まない新しいStringまたはEOF。改行まで消費する |
| `(read-string k [port])` | 1または2、kとテキスト入力 | 最大k文字を消費した新しいString。先にEOFならEOF |
| `(read-u8 [port])` | 0または1、バイナリ入力 | 次の0〜255の正確な整数またはEOF。1バイト進める |
| `(peek-u8 [port])` | 0または1、バイナリ入力 | 次の整数またはEOF。論理的な入力位置を進めない |
| `(u8-ready? [port])` | 0または1、バイナリ入力 | 次のread-u8が待たずに返せれば#t。EOFでも#t |
| `(read-bytevector k [port])` | 1または2、kとバイナリ入力 | 最大kバイトを消費した新しいBytevector。先にEOFならEOF |
| `(read-bytevector! bv [port [start [end]]])` | 1〜4、可変Bytevectorとバイナリ入力、境界 | 指定範囲に読み、読んだバイト数またはEOF。入力位置とbvを変更 |

read-lineはLF、CRとCRLFを改行として扱います。
read-stringとread-bytevectorでkが0なら空の値を返し、入力を消費しません。
read-bytevector!で空範囲を指定した場合も入力を消費せず0を返します。
定数のBytevectorと範囲の誤りは、入力を消費する前に検出します。

ready?は完全な1文字または1バイトが取得できるかを調べます。
UTF-8の先頭バイトだけが到着していてもchar-ready?は#tにしません。
内部で先読みしたバイトは同じポートに保持し、続くpeek／readへ渡します。
Linuxのファイル・パイプ・標準入力は待たないpollで確認し、メモリポートは常に待たずに取得できます。
任意のGo Reader用アダプタは待たないReadyプローブを提供する契約です。
Linux以外の既定ファイルアダプタでは通常ファイルを判定でき、対話ストリームのready?にはOS用のプローブの追加が必要です。
これは内部アダプタの対応状況であり、全OSでの準拠完了を示しません。

```scheme
(define p (open-input-string "ab\r\n猫"))
(list (read-line p) (read-string 2 p) (eof-object? (read-char p)))
; => ("ab" "猫" #t)
```

```scheme
(define p (open-input-bytevector #u8(1 2 3)))
(define b (make-bytevector 4 9))
(read-bytevector! b p 1 3)
b ; => #u8(9 1 2 9)
```

## 出力

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(write-char char [port])` | 1または2、Characterとテキスト出力 | 文字を書き、結果は未規定 |
| `(write-string str [port [start [end]]])` | 1〜4、Stringとテキスト出力、文字の境界 | 指定範囲を書き、結果は未規定 |
| `(newline [port])` | 0または1、テキスト出力 | LFを書き、結果は未規定 |
| `(write-u8 byte [port])` | 1または2、0〜255の正確な整数とバイナリ出力 | バイトを書き、結果は未規定 |
| `(write-bytevector bv [port [start [end]]])` | 1〜4、Bytevectorとバイナリ出力、バイトの境界 | 指定範囲を書き、結果は未規定 |
| `(flush-output-port [port])` | 0または1、出力ポート | バッファの内容を出力先へ渡し、結果は未規定 |

出力手続きはポートを変更し、入力のStringやBytevectorを変更しません。
内部のメモリポートと標準出力アダプタはポート側の出力バッファを設けません。
Go側のWriterがFlushを提供する場合はそれを呼びます。
一部しか書き出せなかった場合は、成功として扱わずエラーを送出します。

```scheme
(define p (open-output-string))
(write-string "a猫b" p 1 2)
(write-char #\λ p)
(flush-output-port p)
(get-output-string p) ; => "猫λ"
```
