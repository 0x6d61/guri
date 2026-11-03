# JSONの読み書き

`(srfi 180)`は、[SRFI 180](https://srfi.schemers.org/srfi-180/srfi-180.html)の12公開束縛を提供します。
通常の読み書き、イベントによるストリーム処理、JSON Lines、JSON Text Sequences、専用例外と読み込み制限に対応します。
Go実装をバイナリに含むため、追加のライブラリファイルは不要です。
REPLでも明示的にimportします。
R7RS-smallの16標準ライブラリと557公開束縛の件数には含めません。

## 値の対応

| JSON | Scheme |
|---|---|
| `null` | Symbolの`'null` |
| `true`、`false` | `#t`、`#f` |
| 整数表記 | 任意精度の正確なInteger |
| 小数点または指数を含む表記 | binary64の不正確な実数 |
| String | Unicode String |
| Array | Vector |
| Object | Symbolをキーとするドット対の関連リスト |

空のArrayは`#()`、空のObjectは`()`です。
Objectのキーの順序と重複を保持します。
たとえば`{"x":1,"x":2}`は`((x . 1) (x . 2))`として読み込みます。
新しいPair、Vector、Stringを作り、入力の文字列とは可変な格納場所を共有しません。
Symbolは通常の`string->symbol`と同じくインターンされます。

小数と指数表記はbinary64へ丸めます。
オーバーフローはJSON例外とし、アンダーフローは符号付きのゼロになる場合があります。
整数表記の`-0`は正確な`0`、`-0.0`は負の不正確なゼロです。
文字列中のUnicodeエスケープを復号し、正しいUTF-16サロゲート対を1文字に変換します。
単独のサロゲート、不正なUTF-8、未エスケープの制御文字、JSONの文法にない数値やエスケープは拒否します。

## 通常の読み書き

| 構文 | 引数数 | 返り値 |
|---|---|---|
| `(json-read [source])` | 0または1 | Scheme値。値の前に入力が尽きた場合はEOF |
| `(json-write value [destination])` | 1または2 | 未規定値 |
| `(json-null? value)` | 1 | `value`がSymbolの`'null`なら`#t`、それ以外は`#f` |

`source`は開いたテキスト入力Port、または呼ぶたびにCharacterかEOFを1値返す引数なしの手続きです。
省略時は呼び出し時の`current-input-port`を使います。
`json-read`は最初の1値だけを読み、同じ入力へ繰り返し呼び出せます。
トップレベルの数値は、後続のJSON値と空白で区切ります。
数値の終わりを調べるときは、区切りの空白を1文字消費する場合があります。
先頭の空白を認めますが、JSONの外側にあるBOMは認めません。

`destination`は開いたテキスト出力Port、またはCharacterとStringを受け付ける引数1個の手続きです。
省略時は呼び出し時の`current-output-port`を使います。
出力手続きにはStringを渡し、返り値は使いません。
`json-write`は入力全体を検証し、出力内容を確定してから書き込みます。
そのため、出力手続きが元の値を変更しても、検証済みの出力は変わりません。

書き込み可能な値は対応表の値です。
非整数の正確な有理数、Complex、無限大、NaN、`'null`以外のSymbol、Character、Bytevector、不正な関連リスト、循環する値はJSON例外になります。
入力が無効な場合は何も出力しません。
循環のない共有データは各出現位置へ書き込み、共有の同一性は保存しません。
出力はコンパクトなJSONで、改行を付けません。
文字列のエスケープ表現は元のJSONと一致するとは限りません。

```scheme
(import (scheme base) (scheme write) (srfi 180))
(define value (json-read (open-input-string "{\"items\":[1,null,true]}")))
(write value)
(newline) ; ((items . #(1 null #t)))
(json-write value)
(newline) ; {"items":[1,null,true]}
```

## イベントによるストリーム処理

| 構文 | 引数数 | 返り値 |
|---|---|---|
| `(json-generator [source])` | 0または1 | 引数なしのイベント生成手続き |
| `(json-fold proc array-start array-end object-start object-end seed [source])` | 6または7 | 最終seed |
| `(json-accumulator destination)` | 1 | イベントを1個受け付ける手続き |

`source`と`destination`の型は通常の読み書きと同じです。
`json-generator`は呼ばれるたびに1イベントを返します。
開始と終了は`'array-start`、`'array-end`、`'object-start`、`'object-end`です。
値は`'null`、Boolean、Number、Stringです。
ObjectのキーはStringとして生成し、その直後に値のイベントを生成します。
1個のトップレベル値が完了するとEOFを返し続け、次のトップレベル値には進みません。
入力全体を先読みせず、開いた構造と現在の単一値を保持します。

`json-fold`の5個の手続きは次の順序で呼び出します。

| 手続き | 呼び出しと役割 |
|---|---|
| `proc` | `(proc value seed)`。単一値、キー、完成した構造をseedへ反映する |
| `array-start`、`object-start` | `(start seed)`。構造内の初期seedを返す |
| `array-end`、`object-end` | `(end seed)`。構造内の最終seedから構造の値を返す |

構造が完成すると、その値と親のseedを`proc`へ渡します。
各手続きは新しいseedまたは構造の値を1値返します。
空の入力では元のseedを返します。
構造を値として保持せず、集計だけを行うこともできます。

`json-accumulator`は1個のトップレベル値のイベント列をJSONへ変換します。
構造の対応、Objectのキーと値の交互出現、単一値の型を検証します。
不正なイベントはJSON例外とし、そのイベントの内容を書き込みません。
その前に受け付けたイベントの出力は残ります。
閉じていない構造の終了を自動補完しないため、呼び出し側が対応する終了イベントを渡します。
完成後の追加イベントやEOFはエラーです。
入力に合わせて逐次書き込む場合はこの手続きを使います。

```scheme
(import (scheme base) (scheme write) (srfi 180))
(define g (json-generator (open-input-string "[1,null]")))
(let loop ((event (g)))
  (unless (eof-object? event)
    (write event) (newline)
    (loop (g))))
; array-start
; 1
; null
; array-end
(define put (json-accumulator (current-output-port)))
(for-each put '(object-start "count" 2 object-end))
(newline) ; {"count":2}
```

## 複数のJSON値

| 構文 | 引数数 | 返り値 |
|---|---|---|
| `(json-lines-read [source])` | 0または1 | Scheme値を返す引数なしの生成手続き |
| `(json-sequence-read [source])` | 0または1 | Scheme値を返す引数なしの生成手続き |

入力の型と省略時のPortは`json-read`と同じで、生成手続きを作るときに入力元を選びます。
各呼び出しは対応表のScheme値を1個返し、入力が尽きるとEOFを返し続けます。
生成手続きは1個の値を組み立てるためのメモリを使います。

`json-lines-read`は`json-read`を繰り返す方式で、改行を含むJSONの空白で値を区切ります。
JSON Linesの1行1値を読み込めますが、行ごとの検証は行いません。
不正なJSONはJSON例外とします。

`json-sequence-read`は[RFC 7464](https://www.rfc-editor.org/rfc/rfc7464)のRS（U+001E）で区切ったJSON Text Sequencesを読み込みます。
最初のRSまでを読み飛ばし、壊れたレコードや途切れたレコードは次のRSまで読み飛ばして復帰します。
トップレベルの数値、Boolean、nullには末尾のJSON空白が必要です。
完全な値とLFを読み終えた時点、次のRS、またはEOFで値を返せるため、後続レコード全体を待ちません。
入出力障害や読み込み制限への到達は、レコードの読み飛ばしで隠さずJSON例外にします。

```scheme
(import (scheme base) (scheme write) (srfi 180))
(define next (json-lines-read (open-input-string "1\n[2]\n")))
(write (list (next) (next) (eof-object? (next))))
(newline) ; (1 #(2) #t)
```

## 読み込み制限とエラー

| 構文 | 引数数 | 返り値 |
|---|---|---|
| `(json-nesting-depth-limit)` | 0 | 最大のネスト深さを表す非負の実数 |
| `(json-number-of-character-limit)` | 0 | 最大の読み込み文字数を表す非負の実数 |
| `(json-error? value)` | 1 | このライブラリの例外なら`#t`、それ以外は`#f` |
| `(json-error-reason exception)` | 1 | JSON例外の理由を示すString |

2個のパラメータは初期値が`+inf.0`で、`parameterize`で変更できます。
負数、NaN、実数以外の値はJSON例外になります。
上限値に達した時点でJSON例外とします。
最上位のArrayまたはObjectの深さは1なので、深さの上限2では`[[]]`を拒否します。
文字数はバイト数ではなくUnicode文字数で、先頭の空白と数値の区切りとして読んだ文字も含みます。
値の途中でEOFになった場合はエラーです。

`json-read`と`json-fold`は呼び出し時、`json-generator`は作成時の上限を使います。
JSON Linesでは各値、JSON Text Sequencesでは各RSからのレコードごとにカウントをリセットします。
これらの生成手続きは各値を読み始める時点の上限を使います。
バイナリPort、方向の異なるPort、閉じたPort、通常手続きの引数の個数と型の誤り、不正なJSON、読み込み制限、読み書きの障害はJSON例外とします。
`json-error-reason`へJSON例外以外を渡した場合もJSON例外です。
理由のStringは例外内に保持したものを返し、`error-object-message`の返り値と共有します。
JSON例外は`error-object?`も満たし、`guard`や`with-exception-handler`で捕捉できます。
入力生成手続き、foldの手続き、出力手続きが送出した例外は、そのまま呼び出し側へ伝えます。

どの手続きも渡されたPortを閉じません。
読み書きの途中の障害では、消費済みの入力や書き込み済みの出力を巻き戻しません。
元のScheme値は変更しませんが、生成手続きと受け付け手続きは自身の進行状態を変更します。
継続を呼び戻しても、すでに行われた入出力やその進行状態を巻き戻しません。
`json-write`は全体を検証するため、出力内容の保持にもメモリを使います。
大きな入力ではアプリケーションに応じて制限パラメータを設定し、値全体が不要な集計には`json-fold`を使えます。

## 設定ファイルの更新例

リポジトリの`examples/json-config.scm`は設定ファイルを読み、`runs`を1増やして別のファイルへ書き込みます。
入力のObjectと末尾のEOFを確認し、複数値や余分な末尾入力を拒否します。
`runs`が重複する場合は最初の値を使い、更新結果では1個にまとめます。
入力のPortと出力のPortは`call-with-input-file`と`call-with-output-file`で管理します。

```sh
printf '%s\n' '{"name":"demo","runs":2}' > /tmp/guri-config.json
./build/guri examples/json-config.scm /tmp/guri-config.json /tmp/guri-config-next.json
# {"runs":3,"name":"demo"}
cat /tmp/guri-config-next.json
# {"runs":3,"name":"demo"}
```

## 検証範囲

独自のunitテストで、型の対応、重複キー、Unicode、数値、末尾入力、イベントの順序、fold、制限、循環と共有、入出力失敗、Portの借用を検証しています。
12,000要素のArray、12,000段のネスト、文字生成手続きを使う集計でも確認しています。
テストにskipやxfailはありません。

追加のパーサー監査では[JSONTestSuite](https://github.com/nst/JSONTestSuite/tree/1ef36fa01286573e846ac449e8683f8833c5b26a)の`test_parsing`を使いました。
固定リビジョンは`1ef36fa01286573e846ac449e8683f8833c5b26a`、出所のライセンスはMIT（Copyright 2016 Nicolas Seriot）です。
外部のソースやfixtureはリポジトリへ取り込まず、検証時に一時ディレクトリへ取得しました。
各fixtureをテキストPortから読み、最初の値の後にEOFになることまで確認しました。
正常系の`y_`95件をすべて受理し、不正系の`n_`188件をすべて拒否しました。
実装依存の`i_`35件は6件受理、29件拒否として別に集計し、必須の成功件数へ含めていません。
この監査はJSONのパーサーに対するもので、SRFIの全手続きやR7RS-smallの監査を代替しません。
