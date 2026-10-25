# 関数・公開束縛一覧

Guri 0.0.3は次の16標準ライブラリを公開CLIからimportできます。
Appendix Aに基づく全557公開束縛を列挙しています。
件数には手続きだけでなく構文・補助キーワードと、異なるライブラリでの同名の再公開を含みます。
機械可読の一覧はリポジトリの`docs/r7rs/exports.csv`です。

## 個別の契約

各リファレンスに構文、引数数、型、返り値、変更の有無、共有・コピー、使用例とエラーを記載しています。

| 分野 | リファレンス |
|---|---|
| 数値塔、算術、比較、数学関数、変換 | [Numbers](https://github.com/0x6d61/guri/wiki/Numbers) |
| Pair、List、Symbol、関連リスト、等価性 | [Pairs-Lists-Symbols](https://github.com/0x6d61/guri/wiki/Pairs-Lists-Symbols) |
| Character、String、Unicode、文字列変換 | [Characters-Strings](https://github.com/0x6d61/guri/wiki/Characters-Strings) |
| 固定長Vector、Bytevector、UTF-8 | [Vectors-Bytevectors](https://github.com/0x6d61/guri/wiki/Vectors-Bytevectors) |
| apply、map、継続、複数値 | [Control-Features](https://github.com/0x6d61/guri/wiki/Control-Features) |
| dynamic-wind | [Dynamic-Wind](https://github.com/0x6d61/guri/wiki/Dynamic-Wind) |
| 例外、error、動的パラメータ | [Exceptions-Parameters](https://github.com/0x6d61/guri/wiki/Exceptions-Parameters) |
| Portと文字・バイト入出力 | [Ports](https://github.com/0x6d61/guri/wiki/Ports) |
| read、write、display、ファイル | [Datum-File-IO](https://github.com/0x6d61/guri/wiki/Datum-File-IO) |
| 循環・共有データの読み書き | [Graph-IO](https://github.com/0x6d61/guri/wiki/Graph-IO) |
| Record | [Records](https://github.com/0x6d61/guri/wiki/Records) |
| Promise | [Lazy-Evaluation](https://github.com/0x6d61/guri/wiki/Lazy-Evaluation) |
| environment、eval、load、REPL、R5RS環境 | [Evaluation-REPL](https://github.com/0x6d61/guri/wiki/Evaluation-REPL) |
| command-line、環境変数、終了、時計 | [System-Interface](https://github.com/0x6d61/guri/wiki/System-Interface) |
| 構文とライブラリ宣言 | [Special-Forms](https://github.com/0x6d61/guri/wiki/Special-Forms)、[Libraries](https://github.com/0x6d61/guri/wiki/Libraries) |

## ライブラリ別の全公開一覧

### `(scheme base)`（238束縛）

`*`, `+`, `-`, `...`, `/`, `<`, `<=`, `=`, `=>`, `>`, `>=`, `_`, `abs`, `and`, `append`, `apply`, `assoc`, `assq`, `assv`, `begin`, `binary-port?`, `boolean=?`, `boolean?`, `bytevector`, `bytevector-append`, `bytevector-copy`, `bytevector-copy!`, `bytevector-length`, `bytevector-u8-ref`, `bytevector-u8-set!`, `bytevector?`, `caar`, `cadr`, `call-with-current-continuation`, `call-with-port`, `call-with-values`, `call/cc`, `car`, `case`, `cdar`, `cddr`, `cdr`, `ceiling`, `char->integer`, `char-ready?`, `char<=?`, `char<?`, `char=?`, `char>=?`, `char>?`, `char?`, `close-input-port`, `close-output-port`, `close-port`, `complex?`, `cond`, `cond-expand`, `cons`, `current-error-port`, `current-input-port`, `current-output-port`, `define`, `define-record-type`, `define-syntax`, `define-values`, `denominator`, `do`, `dynamic-wind`, `else`, `eof-object`, `eof-object?`, `eq?`, `equal?`, `eqv?`, `error`, `error-object-irritants`, `error-object-message`, `error-object?`, `even?`, `exact`, `exact-integer-sqrt`, `exact-integer?`, `exact?`, `expt`, `features`, `file-error?`, `floor`, `floor-quotient`, `floor-remainder`, `floor/`, `flush-output-port`, `for-each`, `gcd`, `get-output-bytevector`, `get-output-string`, `guard`, `if`, `include`, `include-ci`, `inexact`, `inexact?`, `input-port-open?`, `input-port?`, `integer->char`, `integer?`, `lambda`, `lcm`, `length`, `let`, `let*`, `let*-values`, `let-syntax`, `let-values`, `letrec`, `letrec*`, `letrec-syntax`, `list`, `list->string`, `list->vector`, `list-copy`, `list-ref`, `list-set!`, `list-tail`, `list?`, `make-bytevector`, `make-list`, `make-parameter`, `make-string`, `make-vector`, `map`, `max`, `member`, `memq`, `memv`, `min`, `modulo`, `negative?`, `newline`, `not`, `null?`, `number->string`, `number?`, `numerator`, `odd?`, `open-input-bytevector`, `open-input-string`, `open-output-bytevector`, `open-output-string`, `or`, `output-port-open?`, `output-port?`, `pair?`, `parameterize`, `peek-char`, `peek-u8`, `port?`, `positive?`, `procedure?`, `quasiquote`, `quote`, `quotient`, `raise`, `raise-continuable`, `rational?`, `rationalize`, `read-bytevector`, `read-bytevector!`, `read-char`, `read-error?`, `read-line`, `read-string`, `read-u8`, `real?`, `remainder`, `reverse`, `round`, `set!`, `set-car!`, `set-cdr!`, `square`, `string`, `string->list`, `string->number`, `string->symbol`, `string->utf8`, `string->vector`, `string-append`, `string-copy`, `string-copy!`, `string-fill!`, `string-for-each`, `string-length`, `string-map`, `string-ref`, `string-set!`, `string<=?`, `string<?`, `string=?`, `string>=?`, `string>?`, `string?`, `substring`, `symbol->string`, `symbol=?`, `symbol?`, `syntax-error`, `syntax-rules`, `textual-port?`, `truncate`, `truncate-quotient`, `truncate-remainder`, `truncate/`, `u8-ready?`, `unless`, `unquote`, `unquote-splicing`, `utf8->string`, `values`, `vector`, `vector->list`, `vector->string`, `vector-append`, `vector-copy`, `vector-copy!`, `vector-fill!`, `vector-for-each`, `vector-length`, `vector-map`, `vector-ref`, `vector-set!`, `vector?`, `when`, `with-exception-handler`, `write-bytevector`, `write-char`, `write-string`, `write-u8`, `zero?`

### `(scheme case-lambda)`（1束縛）

`case-lambda`

### `(scheme char)`（22束縛）

`char-alphabetic?`, `char-ci<=?`, `char-ci<?`, `char-ci=?`, `char-ci>=?`, `char-ci>?`, `char-downcase`, `char-foldcase`, `char-lower-case?`, `char-numeric?`, `char-upcase`, `char-upper-case?`, `char-whitespace?`, `digit-value`, `string-ci<=?`, `string-ci<?`, `string-ci=?`, `string-ci>=?`, `string-ci>?`, `string-downcase`, `string-foldcase`, `string-upcase`

### `(scheme complex)`（6束縛）

`angle`, `imag-part`, `magnitude`, `make-polar`, `make-rectangular`, `real-part`

### `(scheme cxr)`（24束縛）

`caaaar`, `caaadr`, `caaar`, `caadar`, `caaddr`, `caadr`, `cadaar`, `cadadr`, `cadar`, `caddar`, `cadddr`, `caddr`, `cdaaar`, `cdaadr`, `cdaar`, `cdadar`, `cdaddr`, `cdadr`, `cddaar`, `cddadr`, `cddar`, `cdddar`, `cddddr`, `cdddr`

### `(scheme eval)`（2束縛）

`environment`, `eval`

### `(scheme file)`（10束縛）

`call-with-input-file`, `call-with-output-file`, `delete-file`, `file-exists?`, `open-binary-input-file`, `open-binary-output-file`, `open-input-file`, `open-output-file`, `with-input-from-file`, `with-output-to-file`

### `(scheme inexact)`（12束縛）

`acos`, `asin`, `atan`, `cos`, `exp`, `finite?`, `infinite?`, `log`, `nan?`, `sin`, `sqrt`, `tan`

### `(scheme lazy)`（5束縛）

`delay`, `delay-force`, `force`, `make-promise`, `promise?`

### `(scheme load)`（1束縛）

`load`

### `(scheme process-context)`（5束縛）

`command-line`, `emergency-exit`, `exit`, `get-environment-variable`, `get-environment-variables`

### `(scheme read)`（1束縛）

`read`

### `(scheme repl)`（1束縛）

`interaction-environment`

### `(scheme time)`（3束縛）

`current-jiffy`, `current-second`, `jiffies-per-second`

### `(scheme write)`（4束縛）

`display`, `write`, `write-shared`, `write-simple`

### `(scheme r5rs)`（222束縛）

`*`, `+`, `-`, `...`, `/`, `<`, `<=`, `=`, `=>`, `>`, `>=`, `_`, `abs`, `acos`, `and`, `angle`, `append`, `apply`, `asin`, `assoc`, `assq`, `assv`, `atan`, `begin`, `boolean?`, `caaaar`, `caaadr`, `caaar`, `caadar`, `caaddr`, `caadr`, `caar`, `cadaar`, `cadadr`, `cadar`, `caddar`, `cadddr`, `caddr`, `cadr`, `call-with-current-continuation`, `call-with-input-file`, `call-with-output-file`, `call-with-values`, `car`, `case`, `cdaaar`, `cdaadr`, `cdaar`, `cdadar`, `cdaddr`, `cdadr`, `cdar`, `cddaar`, `cddadr`, `cddar`, `cdddar`, `cddddr`, `cdddr`, `cddr`, `cdr`, `ceiling`, `char->integer`, `char-alphabetic?`, `char-ci<=?`, `char-ci<?`, `char-ci=?`, `char-ci>=?`, `char-ci>?`, `char-downcase`, `char-lower-case?`, `char-numeric?`, `char-ready?`, `char-upcase`, `char-upper-case?`, `char-whitespace?`, `char<=?`, `char<?`, `char=?`, `char>=?`, `char>?`, `char?`, `close-input-port`, `close-output-port`, `complex?`, `cond`, `cons`, `cos`, `current-input-port`, `current-output-port`, `define`, `define-syntax`, `delay`, `denominator`, `display`, `do`, `dynamic-wind`, `else`, `eof-object?`, `eq?`, `equal?`, `eqv?`, `eval`, `even?`, `exact->inexact`, `exact?`, `exp`, `expt`, `floor`, `for-each`, `force`, `gcd`, `if`, `imag-part`, `inexact->exact`, `inexact?`, `input-port?`, `integer->char`, `integer?`, `interaction-environment`, `lambda`, `lcm`, `length`, `let`, `let*`, `let-syntax`, `letrec`, `letrec-syntax`, `list`, `list->string`, `list->vector`, `list-ref`, `list-tail`, `list?`, `load`, `log`, `magnitude`, `make-polar`, `make-rectangular`, `make-string`, `make-vector`, `map`, `max`, `member`, `memq`, `memv`, `min`, `modulo`, `negative?`, `newline`, `not`, `null-environment`, `null?`, `number->string`, `number?`, `numerator`, `odd?`, `open-input-file`, `open-output-file`, `or`, `output-port?`, `pair?`, `peek-char`, `positive?`, `procedure?`, `quasiquote`, `quote`, `quotient`, `rational?`, `rationalize`, `read`, `read-char`, `real-part`, `real?`, `remainder`, `reverse`, `round`, `scheme-report-environment`, `set!`, `set-car!`, `set-cdr!`, `sin`, `sqrt`, `string`, `string->list`, `string->number`, `string->symbol`, `string-append`, `string-ci<=?`, `string-ci<?`, `string-ci=?`, `string-ci>=?`, `string-ci>?`, `string-copy`, `string-fill!`, `string-length`, `string-ref`, `string-set!`, `string<=?`, `string<?`, `string=?`, `string>=?`, `string>?`, `string?`, `substring`, `symbol->string`, `symbol?`, `syntax-rules`, `tan`, `truncate`, `values`, `vector`, `vector->list`, `vector-fill!`, `vector-length`, `vector-ref`, `vector-set!`, `vector?`, `with-input-from-file`, `with-output-to-file`, `write`, `write-char`, `zero?`

## 使用例

```scheme
(import (scheme base) (scheme write))
(define xs (vector 1 2 3))
(vector-set! xs 0 9)
(write (list (vector->list xs) (apply + '(1 2 3))))
(newline) ; ((9 2 3) 6)
```

旧GuriのMap/Keyword操作、`len`、`nth`、`slice`、`println`などは提供しません。
同名でも`append`はListの連結、`assoc`は関連リストの検索、`apply`の最後の引数はListです。
VectorはGo Sliceのヘッダ・容量・共有sliceの契約を持ちません。
旧機能の互換レイヤーや新しい公開Go APIは追加していません。
