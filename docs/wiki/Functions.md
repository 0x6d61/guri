# 関数リファレンス

このページは公開CLIの従来の関数一覧です。
内部VMで利用できる標準の制御手続き6束縛は[継続と複数値](https://github.com/0x6d61/guri/wiki/Control-Features)に記載します。
例外とパラメータの10手続きは[例外と動的パラメータ](https://github.com/0x6d61/guri/wiki/Exceptions-Parameters)に記載します。

このページは現在の全30組み込み関数を説明します。
各関数の引数は、呼び出し前に左から右へ評価します。
`...` は0個以上の繰り返し、`?` は省略可能な引数を示し、実際のソースには書きません。
特殊形式の `defn` や `if` は [特殊形式](https://github.com/0x6d61/guri/wiki/Special-Forms) を参照してください。

## 関数一覧

| 関数 | 呼び出し形式 | 対応する型 | 返り値と変更 |
|---|---|---|---|
| `+` | `(+ number...)` | Integer、Float | 合計。空ならIntegerの0。変更なし |
| `-` | `(- number number...)` | Integer、Float | 減算。1引数なら符号反転。変更なし |
| `*` | `(* number...)` | Integer、Float | 積。空ならIntegerの1。変更なし |
| `/` | `(/ first second number...)` | Integer、Float | 左から除算したFloat。変更なし |
| `inc` | `(inc number)` | Integer、Float | 1を加えた数値。変更なし |
| `=` | `(= first second value...)` | すべての値 | 等価性のBoolean。変更なし |
| `!=` | `(!= a b)` | すべての値 | =の否定のBoolean。変更なし |
| `<` | `(< first second number...)` | Integer、Float | 狭義の昇順かを示すBoolean。変更なし |
| `<=` | `(<= first second number...)` | Integer、Float | 昇順かを示すBoolean。変更なし |
| `>` | `(> first second number...)` | Integer、Float | 狭義の降順かを示すBoolean。変更なし |
| `>=` | `(>= first second number...)` | Integer、Float | 降順かを示すBoolean。変更なし |
| `not` | `(not value)` | すべての値 | 真偽を反転したBoolean。変更なし |
| `get` | `(get map key default?)` | Map、有効なキー型 | 値、未存在ならdefaultまたはnil。変更なし |
| `assoc` | `(assoc collection key value)` | Map、Vector | 浅いコピーを更新して返す |
| `assoc!` | `(assoc! collection key value)` | Map、Vector | 元の要素を更新し、そのコレクションを返す |
| `dissoc` | `(dissoc map key keys...)` | Map | キーを除いた浅いコピーを返す |
| `dissoc!` | `(dissoc! map key keys...)` | Map | 元のMapからキーを除き、同じMapを返す |
| `update` | `(update map key f args...)` | Map、呼び出し可能な値 | 関数の結果で浅いコピーを更新して返す |
| `update!` | `(update! map key f args...)` | Map、呼び出し可能な値 | 関数の結果で元のMapを更新して返す |
| `append` | `(append vector value...)` | Vector | 追加後のVectorヘッダ。配列を再利用する場合がある |
| `conj` | `(conj vector value...)` | Vector | appendの別名。同じ関数値 |
| `nth` | `(nth sequence index)` | Vector／String／Bytes、Integer | 要素、1コードポイントのString、またはバイト値のInteger。変更なし |
| `slice` | `(slice sequence low high)` | Vector／String／Bytes、Integer | 入力と同じ型の部分列。Vectorは基底配列を共有 |
| `len` | `(len value)` | List、Vector、Map、String、Bytes | 要素数、コードポイント数またはバイト長のInteger。変更なし |
| `cap` | `(cap vector)` | Vector | 容量のInteger。変更なし |
| `clone` | `(clone collection)` | Vector、Map | 独立した配列またはMapへの浅いコピー |
| `reduce` | `(reduce f initial vector)` | 呼び出し可能な値、Vector | 畳み込み結果。関数の副作用は発生する |
| `println` | `(println value...)` | すべての値 | 標準出力に表示し、nilを返す |
| `error` | `(error message)` | String | UserErrorで評価を中断。正常な返り値なし |
| `apply` | `(apply f fixed-args... collection)` | 呼び出し可能な値、末尾にVectorまたはList | 展開した引数でfを呼び、その返り値を返す |

引数数が表の形式に合わない場合は、どの関数も `ArityError` を返します。
関数自体が変更を行わない場合でも、引数の評価や渡した関数に副作用があれば実行されます。

## 算術演算

`+`、`-`、`*`、`inc` は、すべてIntegerならIntegerを返します。
Floatを含む場合はFloatで演算します。
`/` は2引数以上を取り、常にFloatを返します。
数値以外を渡すと `TypeError`、Integerのオーバーフロー、ゼロ除算、非有限Float値を生む演算は `ArithmeticError` です。

```clojure
(println (+ 1 2 3)) ; => 6
(println (- 10 2 3)) ; => 5
(println (- 3)) ; => -3
(println (* 2 3 4)) ; => 24
(println (/ 12 2 3)) ; => 2.0
(println (inc 1.5)) ; => 2.5
(println (+) (*)) ; => 0 1
```

## 比較と真偽反転

`=` は2引数以上を取り、すべてが最初の値と等価ならtrueを返します。
`!=` は2引数だけを取り、=の否定を返します。
異なる型の値は原則として等価ではありませんが、IntegerとFloatは正確な数値比較を行います。
コレクションとFunctionの比較規則は [言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification) に従います。

`<`、`<=`、`>`、`>=` は2引数以上を取り、隣り合う数値をすべて比較します。
数値以外を渡すと `TypeError` です。
`not` はnilとfalseに対してtrueを、それ以外にはfalseを返します。

```clojure
(println (= 1 1.0)) ; => true
(println (!= 1 2)) ; => true
(println (< 1 2 3) (<= 1 1 2) (> 3 2 1) (>= 3 3 2)) ; => true true true true
(println (not nil) (not 0)) ; => true false
```

## getとKeyword呼び出し

Mapからキーに対応する値を取得します。
キーがなければnilを返し、defaultを渡した場合はその値を返します。
キーが存在して値がnilの場合は、defaultを使わずnilを返します。
Map以外の対象や、無効なキー型は `TypeError` です。

Keywordは `(:name map)` または `(:name map default)` と呼び出せます。
Keyword呼び出しでも、引数は通常どおり先に評価します。

```clojure
(def user {:name "Alice" :age nil})
(println (get user :name)) ; => Alice
(println (:name user)) ; => Alice
(println (get user :missing "N/A")) ; => N/A
(println (:missing user "N/A")) ; => N/A
(println (get user :age 0)) ; => nil
```

## assocとassoc!

Mapではキーを追加または上書きします。
Vectorでは既存の添字の要素を置き換えます。
一度の呼び出しで指定できるキーと値は1組です。

`assoc` は独立したMapまたは基底配列へ浅くコピーしてから変更します。
`assoc!` は元のMapまたは共有する基底配列を変更します。
いずれも更新後のコレクションを返します。

Vectorの添字はIntegerで、`0 <= index < len` に限ります。
添字の範囲外は `IndexError`、対象、添字、Mapキーの型が不正な場合は `TypeError` です。

```clojure
(def m {:count 0})
(def copied (assoc m :count 1))
(println (:count m) (:count copied)) ; => 0 1
(assoc! m :count 2)
(println (:count m)) ; => 2
(def v [1 2])
(println (assoc v 0 9) v) ; => [9 2] [1 2]
(assoc! v 0 8)
(println v) ; => [8 2]
```

## dissocとdissoc!

1個以上のキーをMapから削除します。
存在しないキーは無視します。
`dissoc` は浅いコピーを変更し、`dissoc!` は元のMapを変更します。
いずれも更新後のMapを返します。
Map以外の対象や無効なキー型は `TypeError` です。

```clojure
(def m {:a 1 :b 2})
(def copied (dissoc m :a))
(println (len m) (len copied)) ; => 2 1
(dissoc! m :a :b :missing)
(println (len m)) ; => 0
```

## updateとupdate!

現在の値を先頭引数としてfを呼び出し、追加のargsをその後に渡します。
キーが未存在なら現在の値としてnilを渡します。
fの返り値をキーに設定し、更新後のMapを返します。

`update` は浅いコピーを、`update!` は元のMapを変更します。
fが失敗した場合、その結果をキーに保存しません。
ただし、f自身が行った副作用を取り消す機能はありません。
対象、キー、fの型が不正なら `TypeError` になり、fの引数エラーや実行時エラーも返します。

```clojure
(def m {:count 1})
(def copied (update m :count inc))
(println (:count m) (:count copied)) ; => 1 2
(update! m :count + 2 3)
(println (:count m)) ; => 6
(println (:new (update {} :new (fn [old] (if old old 7))))) ; => 7
```

## appendとconj

0個以上の値をVectorの末尾へ追加した、新しいSliceヘッダを返します。
元の変数の長さは変わりません。
容量が足りれば基底配列を再利用するため、共有するVectorから変更が見える場合があります。
容量が足りなければ配列を再確保します。
対象がVector以外なら `TypeError` です。

`conj` はappendと同じ関数値を参照する別名です。
どちらかの束縛へ `set!` しても、もう一方の束縛は変わりません。

```clojure
(def a [1 2 3])
(def short (slice a 0 1))
(def extended (append short 9))
(println a) ; => [1 9 3]
(println (len short) (len extended)) ; => 1 2
(set! extended (conj extended 8))
(println extended) ; => [1 9 8]
```

## nth

Vector、String、Bytesの0始まりの添字で要素を取得します。
StringではUnicodeコードポイント単位で1コードポイントのStringを返し、Bytesでは1バイトを0〜255のIntegerで返します。
添字はIntegerで、`0 <= index < len` が必要です。
対象や添字の型が不正なら `TypeError`、範囲外なら `IndexError` です。
List、Mapには対応していません。

```clojure
(println (nth [10 20 30] 1)) ; => 20
(println (nth "日本語" 1)) ; => 本
(println (nth b"Alice" 0)) ; => 65
```

## slice

Vectorの `[low:high]` に相当する部分Vectorを返します。
lowとhighはIntegerで、終了位置の要素を含みません。
基底配列を共有し、返り値の長さは `high - low`、容量は元の容量からlowを引いた値になります。
境界はGoの2添字Sliceと同じく `0 <= low <= high <= cap` です。
対象や境界値の型が不正なら `TypeError`、境界条件を満たさなければ `IndexError` です。

StringではUnicodeコードポイント単位、Bytesではバイト単位で部分列を返します。
この2型の境界は `0 <= low <= high <= len` で、元の値を変更せず同じ型の部分列を返します。

```clojure
(def a [1 2 3])
(def part (slice a 1 3))
(println part (len part) (cap part)) ; => [2 3] 2 2
(assoc! part 0 9)
(println a) ; => [1 9 3]
(println (slice "Alice" 0 3)) ; => Ali
(println (slice "日本語" 1 3)) ; => 本語
(println (slice b"Alice" 0 3)) ; => b"Ali"
```

## lenとcap

`len` はList、Vector、Mapの要素数をIntegerで返します。
StringではUnicodeコードポイント数、Bytesではバイト長です。
結合文字のまとめやUnicode正規化は行いません。
`cap` はVectorの容量をIntegerで返します。
対応する型以外を渡すと `TypeError` です。

```clojure
(println (len '(1 2)) (len [1 2 3]) (len {})) ; => 2 3 0
(println (len "Alice") (len "日本語")) ; => 5 3
(println (len b"日本語")) ; => 9
(println (cap [1 2 3])) ; => 3
```

## clone

Vectorなら独立した基底配列、Mapなら独立したMapへ浅くコピーします。
Vectorのコピーの長さと容量は、元の長さと同じです。
内部に格納したMapやVectorまではコピーしません。
対象がVectorやMap以外なら `TypeError` です。

```clojure
(def a [[1]])
(def b (clone a))
(assoc! (nth b 0) 0 9)
(println a) ; => [[9]]
(def m {:count 1})
(def copied (clone m))
(assoc! copied :count 2)
(println (:count m) (:count copied)) ; => 1 2
```

## reduce

初期値をaccとして、Vectorの先頭から `(f acc element)` を順に呼び出します。
最後のaccを返し、空のVectorなら初期値をそのまま返します。
fが呼び出せない場合や、対象がVector以外の場合は `TypeError` です。
fが返したエラーも伝播します。
reduce自体はVectorを変更しませんが、fの副作用は実行されます。

```clojure
(println (reduce + 10 [1 2 3])) ; => 16
(println (reduce + 10 [])) ; => 10
```

## println

0個以上の値を空白で区切り、最後に改行を出力してnilを返します。
引数なしなら改行だけを出力します。
Stringは引用符を付けず、コレクション内のStringには引用符を付けます。
Mapの表示順序は保証しません。
Functionは `#<function>`、循環参照は `#<cycle>` と表示します。
Bytesは `b"..."` の形式で表示し、非ASCIIや制御バイトをエスケープします。
Go APIでは `Interpreter.Output` で出力先を変更でき、出力先の書き込みエラーは呼び出し元に返します。

```clojure
(println "hello" ["Alice" 1]) ; => hello ["Alice" 1]
```

## error

`(error message)` はStringを1引数取り、指定メッセージのUserErrorで現在の評価を中断します。
正常な返り値はありません。
空のメッセージも許可し、文字列内の `%` はそのまま保持します。
String以外はTypeError、引数数が1個でなければArityErrorになります。
通常の関数なので、引数は先に評価され、別の変数に保持したりコールバックとして渡したりできます。

error自体は出力やプロセス終了を行いません。
関数、while、condなどを通してエラーが伝播し、その後の式は評価しません。
エラー前の副作用は取り消しません。
ファイル名、行、列とメッセージを持つエラーをGo APIへ返し、CLIでは既存の処理で標準エラーへ表示して終了コード1で終了します。
同じInterpreterはエラー後も再利用できます。

```clojure
(error "] に対応する [ がありません。") ; error: UserError: ] に対応する [ がありません。
```

短絡評価で選択されなかった式のerrorは実行しません。

```clojure
(println (if true "safe" (error "not evaluated"))) ; => safe
```

## apply

`(apply f collection)` または `(apply f fixed-args... collection)` と書きます。
fはFunctionまたはKeywordで、最後の引数はVectorまたはListです。
先行する固定引数の後ろへ最後のコレクションの要素を並べ、通常の関数と同じ機構で呼び出します。
最低2引数が必要で、不足するとArityErrorです。
fが呼び出せない場合や最後の引数がVector／List以外ならTypeErrorになります。
最後のnilは空Listとして扱わず、空の引数列には `[]` または `'()` を使います。

```clojure
(println (apply + [1 2 3])) ; => 6
(println (apply + 10 '(1 2 3))) ; => 16
(println (apply + [])) ; => 0
(println (apply :name [{:name "Alice"}])) ; => Alice
(println (apply (fn [first & rest] rest) 1 [2 3])) ; => [2 3]
(println (apply (fn [x] x) '(unknown))) ; => unknown
```

applyの引数自体は通常どおり左から右に評価します。
展開した値を再び式として評価することはありません。
コレクションの要素を引数列へ浅くコピーし、元の引数列は変更しません。
内部のMapやVectorの共有は、通常の関数呼び出しと同じです。
呼び出し先の引数数、型、実行時エラーはそのまま伝播します。
関数本体のエラーは定義元の位置を保持し、転送先の引数エラーなどはapplyの呼び出し位置を付けます。

末尾位置のapplyは呼び出し先へ末尾呼び出しとして転送します。
別名や高階関数経由で呼び出した場合にも、同じ呼び出し機構を使用します。
apply自身をapplyで呼び出す場合も、転送のためのスタックを積み続けません。

```clojure
(defn sum [n acc]
  (if (= n 0) acc (apply sum (- n 1) [(+ acc n)])))
(println (sum 100000 0)) ; => 5000050000
```

- [標準数値手続き](https://github.com/0x6d61/guri/wiki/Numbers): 内部VMの数値塔、比較、整数演算、数学関数、数値と文字列の変換。公開CLIは未接続で、importは内部Runtimeへ接続済み。

- [Pair、List、Symbolと等価性](https://github.com/0x6d61/guri/wiki/Pairs-Lists-Symbols): 内部VMの63手続きと、循環の比較、共有とコピー、mapと探索の継続。

- [CharacterとString](https://github.com/0x6d61/guri/wiki/Characters-Strings): 内部VMの50手続き、Unicodeの分類と大小変換、可変Stringと継続。

- [VectorとBytevector](https://github.com/0x6d61/guri/wiki/Vectors-Bytevectors): 内部VMの27手続き、固定長と浅いコピー、重複copy、UTF-8とコールバック継続。

- [datum labelとグラフの読み書き](https://github.com/0x6d61/guri/wiki/Graph-IO): 内部Reader/Writerの共有・循環と3出力方式。標準read/writeのポート束縛も内部VMへ接続済み。

- [Record型定義](https://github.com/0x6d61/guri/wiki/Records): 内部VMのdefine-record-typeと、生成するコンストラクタ・述語・取得／変更手続き。

- [テキスト・バイナリポート](https://github.com/0x6d61/guri/wiki/Ports): 内部VMの38束縛、開閉と借用、文字／バイトの入出力、current portsと継続。datumのread/write、ファイルと公開CLIは後続。

- [datumとファイルの入出力](https://github.com/0x6d61/guri/wiki/Datum-File-IO): 内部VMの13手続き、ストリームReader、write/display、ファイルの所有と継続。公開CLIは後続。importは内部Runtimeへ接続済み。

- [ライブラリとプログラムの環境](https://github.com/0x6d61/guri/wiki/Libraries): define-library/import、lib/の検索規則、9標準ライブラリと未提供の7ライブラリ、初期化と継続。
