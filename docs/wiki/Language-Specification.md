# Guri 0.0.3の言語仕様

R7RS-smallを目標仕様とするScheme処理系です。
GoのバイトコードVMでシングルスレッド実行し、動的・強い型付けとレキシカルスコープを持ちます。
公開CLIは標準Reader、コンパイラ、VMと16標準ライブラリに接続済みです。
準拠宣言と未確認範囲は[監査結果](https://github.com/0x6d61/guri/wiki/R7RS-Audit)を参照してください。

## 値と記法

| 型 | 例・規則 | リファレンス |
|---|---|---|
| Boolean | `#t`、`#f`。偽値は`#f`だけ | [値層](https://github.com/0x6d61/guri/wiki/Runtime-Data-Model) |
| 空List・Pair | `'()`、`'(1 2)`、`'(1 . 2)`。空ListとBooleanは別の値 | [Pair/List](https://github.com/0x6d61/guri/wiki/Pairs-Lists-Symbols) |
| 数値 | 任意精度整数、正確な有理数、binary64、正確・不正確な複素数 | [数値](https://github.com/0x6d61/guri/wiki/Numbers) |
| Character | `#\a`、`#\space`、`#\x672c` | [Character/String](https://github.com/0x6d61/guri/wiki/Characters-Strings) |
| String | `"日本語"`。Unicodeコードポイント単位。可変な生成値と変更不可のリテラルを区別 | [Character/String](https://github.com/0x6d61/guri/wiki/Characters-Strings) |
| Symbol | `'name`、`'|two words|` | [Symbol](https://github.com/0x6d61/guri/wiki/Pairs-Lists-Symbols) |
| Vector | `#(1 2)`。固定長。`vector`で作った値は可変 | [Vector](https://github.com/0x6d61/guri/wiki/Vectors-Bytevectors) |
| Bytevector | `#u8(0 255)`。固定長のバイト列 | [Bytevector](https://github.com/0x6d61/guri/wiki/Vectors-Bytevectors) |
| Procedure | `(lambda (x) x)`。クロージャ、継続、組み込み手続き | [制御](https://github.com/0x6d61/guri/wiki/Control-Features) |
| Record | `define-record-type`で定義する生成的な型 | [Record](https://github.com/0x6d61/guri/wiki/Records) |
| Port | テキスト・バイナリ、入力・出力。資源の所有と借用を区別 | [Port](https://github.com/0x6d61/guri/wiki/Ports) |
| Promise | `delay`、`delay-force`で作る遅延値 | [Promise](https://github.com/0x6d61/guri/wiki/Lazy-Evaluation) |

Map、Keyword、特別なnil、Go Slice互換のVector、不変Bytesは提供しません。
`nil`、`true`、`false`、`:name`など、正当なScheme識別子は利用者が定義できます。

## Readerと評価

UTF-8ソースとR7RSのdatumを読み取ります。
`;`コメント、`#| ... |#`、`#;`、引用と準引用、datum label、fold-case directiveを扱います。
通常のList記法は`(...)`、Vectorは`#(...)`、Bytevectorは`#u8(...)`です。
詳細とReaderエラーは[標準Reader](https://github.com/0x6d61/guri/wiki/R7RS-Reader)、循環・共有データは[グラフ入出力](https://github.com/0x6d61/guri/wiki/Graph-IO)を参照してください。

名前はレキシカルな束縛を検索し、クロージャは外側の束縛セルを共有します。
手続き呼び出しは引数を評価してから呼び出します。
オペランドの評価順序など規格が未規定とする結果に、利用者のコードが依存しないようにします。
末尾呼び出しはVMの継続を増やさず実行し、`apply`や継続の再開にも対応します。
`call/cc`、複数値、`dynamic-wind`、例外と動的パラメータを利用できます。

```scheme
(import (scheme base) (scheme write))
(define (make-counter)
  (let ((n 0))
    (lambda () (set! n (+ n 1)) n)))
(define next (make-counter))
(write (list (next) (if '() 'true 'false)))
(newline) ; (1 true)
```

## 可変性とコピー

Pair、String、Vector、Bytevectorは、変数への代入で同じオブジェクトを共有します。
リテラル定数の変更はエラーです。
`list-copy`、`string-copy`、`vector-copy`、`bytevector-copy`などのコピー手続きを使用します。
Vectorの長さ・容量の区別や基底配列を共有するsliceはありません。
`vector-copy`は独立した要素領域への浅いコピーです。
等価性は`eq?`、`eqv?`、`equal?`を目的に応じて使い分けます。

```scheme
(import (scheme base) (scheme write))
(define a (vector 1 2 3))
(define b a)
(vector-set! b 0 9)
(define c (vector-copy a))
(vector-set! c 0 100)
(write (list a c))
(newline) ; (#(9 2 3) #(100 2 3))
```

## エラーと実行環境

Schemeの例外は`raise`、`raise-continuable`、`error`、`guard`などで扱います。
Reader、束縛解決、型、引数数、範囲、入出力のエラーでは可能な範囲でファイル名・行・列と原因を示します。
旧GuriのGoエラー型は公開契約として残しません。
[例外](https://github.com/0x6d61/guri/wiki/Exceptions-Parameters)、[プログラムとライブラリ](https://github.com/0x6d61/guri/wiki/Libraries)、[CLI](https://github.com/0x6d61/guri/wiki/CLI)に詳細を記載します。

## 実装範囲と制限

binary64以外の任意の浮動小数点精度指定は未採用です。
数値の非有限値や除算は旧GuriのArithmeticError規則を継承せず、標準の数値塔の仕様に従います。
#95でLinux、macOS、WindowsのポートReadyとOS別CIを検証しました。
確認した環境と結果は[結合監査](https://github.com/0x6d61/guri/wiki/R7RS-Audit)に記録します。
全16標準ライブラリの557公開束縛の一覧と個別契約は[関数一覧](https://github.com/0x6d61/guri/wiki/Functions)を参照してください。
