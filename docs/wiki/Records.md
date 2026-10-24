# SchemeのRecord型定義

`define-record-type`を内部の`core.NewCore`環境へ接続しています。
公開CLIと標準ライブラリのimportは未接続です。
この形式は[R7RS-small §5.5](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-7.html#TAG:__tex2page_sec_5.5)に対応します。
旧Mapや独自のフィールド操作を代用せず、既存のRecordとRecordTypeを使います。

## 構文と定義される束縛

```scheme
(define-record-type pare
  (kons x y)
  pare?
  (x kar set-kar!)
  (y kdr))
(define value (kons 1 2))
(pare? value)       ; => #t
(kar value)         ; => 1
(set-kar! value 3)
(kar value)         ; => 3
(kdr value)         ; => 2
```

構文は`(define-record-type name (constructor field-name...) predicate field...)`です。
name、constructor、predicateと各フィールドの名前は識別子です。
各fieldは`(field-name accessor)`または`(field-name accessor mutator)`です。
フィールドは0個以上で、コンストラクタが受け取るフィールドも0個以上です。
定義はトップレベルか本体の先頭の定義領域で使えます。
式の位置には置けません。

| 束縛 | 引数数と型 | 返り値と変更 |
|---|---|---|
| name | 型を表す構文識別子 | この実装では規格が許す純粋な構文表現を使う。通常の値や手続きではない |
| constructor | コンストラクタ節のfield-nameと同じ個数。各引数は任意の値 | 新しいRecord。既存の値は変更しない |
| predicate | 1、任意 | この定義が生成した型のRecordなら#t、それ以外は#f |
| accessor | 1、この定義が生成した型のRecord | 対応するフィールドの現在の値そのもの。変更なし |
| mutator | 2、同じ型のRecordと任意の値 | 元のフィールドを変更。結果は未規定 |

mutatorを省略したフィールドには変更手続きを作りません。
コンストラクタの引数順はフィールド宣言の順と異なっていても構いません。
コンストラクタ節にないフィールドの初期値は未規定です。
コンストラクタは渡した可変オブジェクトを深くコピーせず、同じ参照を格納します。

```scheme
(define-record-type point
  (make-point y x)
  point?
  (x point-x)
  (y point-y)
  (label point-label set-point-label!))
(define p (make-point 2 1))
(point-x p)                 ; => 1
(point-y p)                 ; => 2
(set-point-label! p "home")
(point-label p)             ; => "home"
```

## 型の生成と共有

定義を実行するたびに、既存の全ての型から区別されるRecordTypeを作ります。
名前やフィールド構造が同じでも、別の実行で作った型のRecordを取り違えません。
関数の中の同じ定義も、呼び出しごとに新しい型を作ります。
コンパイル時に一つの型を作って全ての実行で使い回しません。

Recordの代入は同じオブジェクトを参照します。
この実装のeq?、eqv?とequal?はRecordの参照同一性を調べます。
別々に生成したRecordのフィールド内容が同じでも、この実装ではequal?を#fにします。
別Recordに対するequal?のこの結果を、全処理系共通の必須結果とは扱いません。

```scheme
(define (factory)
  (define-record-type item (make-item) item?)
  (list make-item item?))
(define a (factory))
(define b (factory))
((cadr a) ((car a))) ; => #t
((cadr a) ((car b))) ; => #f
```

## レキシカルな定義とマクロ

内部定義は既存の定義領域へ展開し、構築用の一時変数には独立した文脈を与えます。
nameは構文識別子として導入し、コンストラクタ、述語、取得と変更の手続きは変数として定義します。
利用者がdefineやsyntax-rulesなどを隠していても、内部で生成する構文を取り違えません。
フィールドの対応も識別子の文脈で判定するため、マクロが導入した同じ綴りの名前を誤って一つにまとめません。

実行時には型の名前、フィールドのメタデータと引数の対応を使います。
型生成やフィールド操作のためにASTを評価しません。

## エラーと未接続部分

必須部分の欠落、不正な識別子、重複するフィールドや束縛名、存在しないコンストラクタフィールドはSyntaxErrorです。
同じフィールドをコンストラクタで複数回指定する場合も診断します。
accessorとmutatorに違う型のRecordやRecord以外を渡すと、error-objectを非継続可能に送出します。
guardで捕捉できます。
生成した手続きの引数数が違う場合はVMの引数数エラーであり、自動送出にはまだ接続していません。

```scheme
(define-record-type entry (make-entry x) entry? (x entry-x))
(guard (e (else #t)) (entry-x 1) #f) ; => #t
```

`internal/core/records_test.go`で生成ごとの型、順序、省略フィールド、可変性、型検査、内部定義とマクロを検証します。
このページの10使用例を内部環境で実行します。
Recordの読み戻し可能な独自表記やリフレクション手続きは追加しません。
標準ライブラリと公開CLIへの接続は後続Issueであり、内部の成功と公開CLIの適合測定を区別します。
