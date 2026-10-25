# 構文リファレンス

Guri 0.0.3ではR7RS-smallの構文を使用します。
ファイルのプログラムは最初に必要なライブラリをimportします。
手続きの呼び出しと異なり、構文は各サブ式の評価、束縛領域、末尾位置を規定します。
構文自体の引数は評価前の式であり、通常の値の型に制限されません。
不正な形、束縛、重複や未初期化参照は構文・定義エラーです。

| 分野 | 構文 | 契約と使用例 |
|---|---|---|
| 基本 | `quote`、`lambda`、`if`、`set!`、`begin`、`define`、`let`、`let*`、`letrec`、`letrec*` | [コア構文](https://github.com/0x6d61/guri/wiki/Core-Syntax) |
| マクロ | `define-syntax`、`let-syntax`、`letrec-syntax`、`syntax-rules`、`syntax-error` | [マクロ](https://github.com/0x6d61/guri/wiki/Syntax-Rules) |
| 条件・反復 | `cond`、`case`、`and`、`or`、`when`、`unless`、`do`、`case-lambda` | [派生構文](https://github.com/0x6d61/guri/wiki/Derived-Syntax) |
| 準引用 | `quasiquote`、`unquote`、`unquote-splicing` | [派生構文](https://github.com/0x6d61/guri/wiki/Derived-Syntax) |
| 複数値 | `define-values`、`let-values`、`let*-values` | [制御機能](https://github.com/0x6d61/guri/wiki/Control-Features) |
| 動的束縛・例外 | `parameterize`、`guard` | [例外とパラメータ](https://github.com/0x6d61/guri/wiki/Exceptions-Parameters) |
| Record | `define-record-type` | [Record](https://github.com/0x6d61/guri/wiki/Records) |
| 遅延 | `delay`、`delay-force` | [Promise](https://github.com/0x6d61/guri/wiki/Lazy-Evaluation) |
| ソース・機能 | `include`、`include-ci`、`cond-expand` | [派生構文](https://github.com/0x6d61/guri/wiki/Derived-Syntax) |
| プログラム | `import`、`define-library`とその宣言 | [ライブラリ](https://github.com/0x6d61/guri/wiki/Libraries) |

各参照先に、構文の形、個数、返り値、束縛と共有、使用例、エラーを記載しています。
ライブラリ別の補助キーワードを含む全公開一覧は[Functions](https://github.com/0x6d61/guri/wiki/Functions)です。

## 関数定義と可変長引数

`(define (name args ...) body ...)`は手続き定義の省略形です。
`(lambda (a . rest) body ...)`の残りの引数はListになります。
`define`の返り値や`set!`の返り値は未規定であり、計算結果として使いません。

```scheme
(import (scheme base) (scheme write))
(define (sum first . rest) (apply + first rest))
(display (sum 1 2 3))
(newline) ; 6
```

## 分岐と反復

`cond`の各節は`(test expression ...)`です。
`else`と`=>`は規定された位置で使います。
`do`の反復変数は終端判定後に本体を実行し、stepを同時に更新します。

```scheme
(import (scheme base) (scheme write))
(define (sign x)
  (cond ((< x 0) 'negative)
        ((= x 0) 'zero)
        (else 'positive)))
(write (sign 3))
(newline) ; positive
(do ((i 0 (+ i 1))) ((= i 3))
  (display i))
(newline) ; 012
```

`def`、`defn`、`fn`、`while`、平坦な独自`cond`は提供しません。
