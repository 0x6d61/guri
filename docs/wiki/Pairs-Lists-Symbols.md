# Pair、List、Symbolと等価性

このページの63手続きは内部の`core.NewCore`環境に登録しています。
公開CLI、標準ライブラリのimportと公開Go APIの切替は後続作業です。
公開CLIの従来のVector用appendやMap用assocを、ここに記載する標準の手続きと混同しないでください。
参照先は[R7RS-small §6.1、§6.3、§6.4、§6.5、§6.10](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html)です。
base、cxrとr5rsの公開束縛への割り当ては`docs/r7rs/exports.csv`で追跡します。

以下の`k`は非負の正確な整数、`obj`は任意の値です。
`...`は繰り返し、`[arg]`は省略可能な引数を表す説明用の記号です。
表に変更を記載しない手続きは引数を変更しません。
引数型の不一致、範囲外、変更できないリテラル、要求するList形状の不一致はerror-objectを非継続可能に送出し、guardで捕捉できます。
引数数の不一致はVMの引数数エラーで、例外の自動送出にはまだ接続していません。

## 等価性とBoolean

| 手続き | 引数数・型 | 返り値 |
|---|---|---|
| `(eq? a b)`、`(eqv? a b)` | 2、任意 | 同値性のBoolean |
| `(equal? a b)` | 2、任意 | 内容の等価性のBoolean |
| `(boolean? obj)` | 1、任意 | Booleanなら#t |
| `(not obj)` | 1、任意 | objが#fなら#t、それ以外は#f |
| `(boolean=? b1 b2 b...)` | 2以上、Boolean | すべて同じBooleanなら#t |

eqv?はPair、Vector、Bytevector、String、Record、PortとProcedureの参照同一性を調べます。
Symbolは名前、Characterはコードポイントで比較します。
数値は正確性と符号付きゼロを区別し、NaNの扱いは[数値手続き](https://github.com/0x6d61/guri/wiki/Numbers)に記載します。
eq?は規格が許す範囲でeqv?と同じ実装を使います。
数値やCharacterのeq?結果など、他の処理系で結果が異なってよい場合を移植可能な必須結果とは扱いません。

等価な参照を返す場合を除き、equal?はPair、Vector、StringとBytevectorの内容を比較します。
循環や共有を含む有限のオブジェクトグラフでも停止し、共有方法が異なっても展開した木の内容が同じなら#tです。
それ以外の型はeqv?と同じ比較です。
比較中にSchemeのコールバックは呼びません。

```scheme
(list (eq? 'a 'a) (eqv? 1 1.0) (equal? '(1) '(1))) ; => (#t #f #t)
(list (not #f) (not '()) (boolean=? #t #t))         ; => (#t #f #t)
```

## Pairの生成と更新

| 手続き | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(pair? obj)`、`(null? obj)`、`(list? obj)` | 1、任意 | Pair／空List／空Listで終わる有限のproper Listなら#t |
| `(cons a d)` | 2、任意 | 新しい可変Pair。aとdを浅く保持 |
| `(car pair)`、`(cdr pair)` | 1、Pair | car／cdrに格納した値そのもの |
| `(set-car! pair obj)`、`(set-cdr! pair obj)` | 2、Pairと任意の値 | 元のPairを更新。返り値は未規定 |

空ListはPairではありません。
cdrは任意の値でよく、improper Listや循環を作れます。
list?は循環やimproper Listでは#fです。
consで格納した可変オブジェクトはコピーせず、同じ参照を保持します。
リテラルから作ったPairの変更はエラーです。

```scheme
(cons 1 2) ; => (1 . 2)
(define x (list 1 2))
(set-car! x 9)
x          ; => (9 2)
(set-cdr! x x)
(list? x)  ; => #f
```

## carとcdrの合成

次の30手続きは1個の値を受け取り、名前のa/dを右から順にcar/cdrとして適用した値を返します。
途中の値がPairでなければエラーです。
コピーも変更もせず、格納したオブジェクトを返します。
carとcdrは上の表にも記載しています。

- 1段: `car`、`cdr`
- 2段: `caar`、`cadr`、`cdar`、`cddr`
- 3段: `caaar`、`caadr`、`cadar`、`caddr`、`cdaar`、`cdadr`、`cddar`、`cdddr`
- 4段: `caaaar`、`caaadr`、`caadar`、`caaddr`、`cadaar`、`cadadr`、`caddar`、`cadddr`、`cdaaar`、`cdaadr`、`cdadar`、`cdaddr`、`cddaar`、`cddadr`、`cdddar`、`cddddr`

```scheme
(cadddr '(0 1 2 3))  ; => 3
(caaadr '((0) ((42)))) ; => 42
```

## Listの操作

| 手続き | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(list obj...)` | 0以上、任意 | 引数を格納する新しい可変List。空なら() |
| `(make-list k [fill])` | 1または2、kと任意の値 | k個の新しいPair。fillを浅く共有。省略時の要素は未規定 |
| `(length list)` | 1、proper List | 要素数の正確な整数 |
| `(append list...)` | 0以上、最後以外はproper List | 前のListのPairを浅くコピーして最後の引数へ連結 |
| `(reverse list)` | 1、proper List | 順を反転した新しい可変List |
| `(list-tail obj k)` | 2、cdrをk回たどれる値とk | 到達した値そのもの |
| `(list-ref list k)` | 2、k番目にPairがある値とk | そのPairのcar |
| `(list-set! list k obj)` | 3、同じ条件と任意の値 | そのPairのcarを更新。返り値は未規定 |
| `(list-copy obj)` | 1、任意 | cdrに沿うPairだけを浅くコピー。循環ではエラー |

appendの最後の引数は任意で、その構造を共有します。
0引数なら()、1引数ならその値そのものを返します。
最後がimproper Listなら結果もimproper Listです。
list-copyはimproper Listの最後のcdrを共有し、Pair以外はそのまま返します。
list、make-list、reverseとlist-copyが新しく作ったPairは可変です。
格納した要素自体を再帰的にコピーしません。

list-tail、list-refとlist-set!は循環Listでも有限の添字を扱えます。
循環の周期を使うため、任意精度整数の大きな添字でも、その値だけの回数を反復しません。
make-listで実際に確保できる要素数にはGoのアドレス空間と利用可能メモリの制約があります。
負の添字、Floatの添字、短すぎるList、lengthやreverseのimproper／循環Listはエラーです。

```scheme
(append '(1 2) '(3 . 4)) ; => (1 2 3 . 4)
(list-copy '(1 2 . 3))   ; => (1 2 . 3)
(list-tail '(1 2 . 3) 2) ; => 3
(define tail (list 3))
(define joined (append '(1 2) tail))
(set-car! tail 9)
joined                   ; => (1 2 9)
```

## 探索

| 手続き | 引数数・型 | 返り値 |
|---|---|---|
| `(memq obj list)`、`(memv obj list)` | 2、任意とList | eq?／eqv?で一致した最初の要素から始まるtail。なければ#f |
| `(member obj list [compare])` | 2または3、任意、Listと2引数手続き | compare、またはequal?で一致したtail。なければ#f |
| `(assq obj alist)`、`(assv obj alist)` | 2、任意とPairのList | 各Pairのcarをeq?／eqv?で調べ、一致した元のPair。なければ#f |
| `(assoc obj alist [compare])` | 2または3、任意、PairのListと2引数手続き | compare、またはequal?で一致した元のPair。なければ#f |

compareは検索値、格納した値の順で呼び、単一の真値を返した時点で探索を止めます。
真値は#f以外の値です。
結果のPairやtailをコピーせず、元のListと共有します。
compareの副作用は発生し、同じ動的環境で実行します。
比較中のcall/ccで保存した継続を複数回再開できます。
不正なalist要素、Pair以外の途中のcdr、終端に到達しない循環を検出した場合はエラーです。

```scheme
(memv 2 '(1 2 3))                  ; => (2 3)
(member '(2) '((1) (2) (3)))        ; => ((2) (3))
(assoc 2.0 '((1 a) (2 b)) =)        ; => (2 b)
```

## mapとfor-each

| 手続き | 引数数・型 | 返り値と副作用 |
|---|---|---|
| `(map proc list list...)` | 2以上、手続きと1個以上のList | 対応する要素へprocを適用した結果を格納する新しい可変List |
| `(for-each proc list list...)` | 2以上、同じ型 | 先頭から順にprocを適用。結果は未規定 |

最も短いListが終わると停止します。
循環Listを混ぜられますが、すべてが循環の場合はエラーです。
各呼び出しの引数数はListの個数と同じです。
mapのprocは単一値を返す必要があり、for-eachでは結果を捨てます。
procが入力Listを変更するのは規格上エラーです。
mapの適用順は規格上未規定ですが、この実装では先頭から順に適用します。

SchemeのコールバックをGoの再帰評価で実行せず、VMのInvoke/Resume契約を使います。
途中で保存した継続を再開しても、以前返した結果のPairは変更しません。
結果に格納する可変オブジェクトは、手続きが返した参照を浅く保持します。

```scheme
(map + '(1 2 3) '(4 5)) ; => (5 7)
(define total 0)
(for-each (lambda (x) (set! total (+ total x))) '(1 2 3))
total                    ; => 6
```

## Symbol

| 手続き | 引数数・型 | 返り値 |
|---|---|---|
| `(symbol? obj)` | 1、任意 | Symbolなら#t |
| `(symbol=? s1 s2 s...)` | 2以上、Symbol | 名前がすべて同じなら#t |
| `(symbol->string symbol)` | 1、Symbol | 名前をそのまま格納した不変のString |
| `(string->symbol string)` | 1、String | 共通Symbol表で名前をinternしたSymbol |

Symbolの名前は不変で、Unicodeの名前の大文字と小文字を区別します。
symbol->stringの結果に変更を加えるとエラーです。
string->symbolは入力Stringの可変領域を共有しません。

```scheme
(symbol=? 'a (string->symbol "a")) ; => #t
(symbol->string '|雪 山|)          ; => "雪 山"
```

## 検証と未接続部分

`internal/core/pairs_test.go`で上記の操作、使用例、共有、リテラル変更、循環、コールバック継続と長いListを検証します。
`internal/equality`ではcar方向の循環、共有形状の違いと10万段の深い構造を検証します。
Vectorの循環比較は内部VM側で検証します。
公開CLIの適合ケース、importと標準ライブラリの公開は別の後続作業です。
