# 言語仕様

GuriはGoで実装した、シングルスレッドのASTインタプリタです。
動的かつ強い型付けを採用し、LispのS式を使います。
このページは初期仕様v0.0.1に `defn`、String／Bytesの区別、while、condを追加した現在の実装を説明します。

## データ型

| 型 | 記法 | 性質 |
|---|---|---|
| Nil | `nil` | 単一値 |
| Boolean | `true`、`false` | 真偽値 |
| Integer | `42`、`-3` | 符号付き64ビット整数 |
| Float | `3.14`、`1e3` | 有限の64ビット浮動小数点数 |
| String | `"hello"` | Unicode文字列、不変 |
| Bytes | `b"hello"` | 任意のバイト列、不変 |
| Symbol | `name` | 変数名や識別子 |
| Keyword | `:name` | 自己評価する識別子。Mapを引数に呼び出せる |
| List | `'(1 2 3)` | 連結構造は不変 |
| Vector | `[1 2 3]` | 可変。GoのSliceに準拠 |
| Map | `{:name "Alice"}` | 可変。GoのMapに準拠 |
| Function | `(fn [x] x)` | 第一級関数。レキシカルクロージャ |

Listの連結構造は変更できません。
内部に格納したMapやVectorは可変のまま保持します。
Stringは暗黙に数値へ変換されず、たとえば `(+ "1" 2)` は `TypeError` です。

## Reader

UTF-8のソースを読み取ります。
空白で要素を区切り、`;` から行末まではコメントになります。
`(...)` はList、`[...]` はVector、`{key value ...}` はMapです。
`'expr` は `(quote expr)` と同じ意味です。

文字列で使用できるエスケープは `\"`、`\\`、`\n`、`\r`、`\t` です。
閉じられていない文字列、不明なエスケープ、括弧の不一致、奇数個のMap要素、不正なUTF-8は `ReaderError` になります。
Integerの範囲外リテラルと、Floatの範囲を超えるリテラルも `ReaderError` になります。
実行時の算術演算によるオーバーフローは `ArithmeticError` です。

Bytesリテラルは `b"..."` と書き、Stringと同じエスケープに加えて `\xHH` を使用できます。
HHは正確に2桁の16進数で、たとえば `b"\x00\xff"` は2バイトです。
ソース内に直接書いたUnicode文字はUTF-8のバイト列として格納します。
Bytesのエスケープが不正な場合も `ReaderError` になります。

## 評価順と真偽

トップレベルの式を上から順に評価し、最後の値を返します。
空入力はnilです。
CLIでは返り値を自動表示せず、`println` を呼び出したときに出力します。

| 式 | 評価 |
|---|---|
| 数値、String、Bytes、Boolean、nil、Keyword | その値を返す |
| Symbol | 環境から名前を検索する |
| `()` | 空Listを返す |
| `(f a b)` | f、a、bの順に評価して呼び出す |
| `[a b]` | 左から要素を評価してVectorを作る |
| `{key value ...}` | 記述順にキー、値を評価してMapを作る |
| `'expr` | 内部を評価せずデータとして返す |

偽値はnilとfalseだけです。
`0`、`""`、`[]`、`{}`、`'()` は真値です。
`if`、`and`、`or` は必要な式だけを評価します。
`cond` は条件と式を交互に取り、最初の真値に対応する式だけを評価します。
条件が一致しない場合や空の場合はnilを返し、奇数引数は評価前にArityErrorになります。
`while` は各反復前に条件を評価し、真値の間、本体を順に実行します。
条件が偽値になったらnilを返し、反復ごとのスコープは作りません。
反復回数に比例してスタックを増やしません。

## 束縛と関数

`def` と `defn` はトップレベルの環境で名前を定義します。
同一スコープの再定義は `DefinitionError` になります。
既存の束縛の更新には `set!` を使います。
`let` はローカルな束縛を順に作成します。

関数は定義時の環境を保持し、外側の束縛セルを共有します。
呼び出した場所の同名変数に置き換わることはありません。
本体は順に評価して最後の値を返し、空の本体はnilを返します。
`[a & rest]` の残りの引数はVectorになります。
引数の型は宣言しません。

```clojure
(defn make-counter []
  (let [n 0]
    (fn [] (set! n (inc n)))))
(def next (make-counter))
(println (next)) ; => 1
(println (next)) ; => 2
```

## 等価性とMapキー

`=` はIntegerとFloatを数値として比較します。
異種数値の比較では、IntegerをFloatへ変換する丸めで誤判定しません。
List、Vector、Mapは型と内容で比較し、Mapのキーの並び順は問いません。
Functionは参照同一性で比較します。

```clojure
(println (= 1 1.0)) ; => true
(println (= 9007199254740993 9007199254740992.0)) ; => false
(println (= [1] '(1))) ; => false
(println (= {:a 1 :b 2} {:b 2 :a 1.0})) ; => true
```

Mapのキーに使える型はNil、Boolean、Integer、Float、String、Bytes、Symbol、Keywordです。
StringとBytesは暗黙に変換せず、等価性とMapのキー検索でも区別します。
キー検索はGoのMapと同じく型を区別するため、`1` と `1.0` は別のキーになります。
Mapの等価性でも、キーはこの検索規則で対応付けます。
`=` による数値比較と、Mapのキー検索は規則が異なります。

```clojure
(def m {1 "integer" 1.0 "float"})
(println (get m 1)) ; => integer
(println (get m 1.0)) ; => float
```

## StringとBytes

Stringの `len`、`nth`、`slice` はUnicodeコードポイント単位です。
`nth` は1コードポイントのString、`slice` は部分Stringを返します。
StringをUTF-8の途中で分割しません。
コードポイントは表示上の1文字と異なる場合があり、結合文字のまとめやUnicode正規化は行いません。

Bytesの `len`、`nth`、`slice` はバイト単位です。
`nth` は0〜255のInteger、`slice` はBytesを返します。
どちらの型も不変で、負の添字を許しません。
`nth` の境界は `0 <= index < len`、`slice` の境界は `0 <= low <= high <= len` です。

初期仕様v0.0.1ではStringのlenをバイト長としていましたが、この仕様ではコードポイント数へ変更しています。

```clojure
(println (len "日本語") (len b"日本語")) ; => 3 9
(println (nth "日本語" 1)) ; => 本
(println (slice "A😀B" 1 2)) ; => 😀
(println (nth b"Alice" 0)) ; => 65
(println (slice b"Alice" 0 3)) ; => b"Ali"
(println (= "A" b"A")) ; => false
```

## Vectorの共有

Vectorは基底配列、長さ、容量を持ちます。
代入や関数への受け渡しではヘッダをコピーし、基底配列を共有します。
リテラルの初期容量は要素数と同じです。

`assoc!` は共有する配列の要素を変更します。
`append` は新しいヘッダを返し、容量が足りれば基底配列を再利用します。
元の変数の長さは変わらないため、長さを更新するには返り値を代入します。

```clojure
(def a [1 2 3])
(def b a)
(assoc! b 0 9)
(println a) ; => [9 2 3]
(set! b (append b 4))
(println (len a) (len b)) ; => 3 4
```

`slice` は配列を共有し、範囲は `0 <= low <= high <= cap` です。
長さを超えていても容量以内の領域を参照できます。
`nth` と `assoc`、`assoc!` の添字は長さ以内に制限されます。
`assoc` と `clone` は独立した配列へ要素を浅くコピーします。

## Mapの共有

Mapの代入では本体を共有します。
`assoc!`、`dissoc!`、`update!` は元のMapを変更します。
`assoc`、`dissoc`、`update`、`clone` は浅いコピーを返します。
コピーしたMap内のVectorやMapは引き続き共有します。
列挙順序は保証しません。

## エラー

| エラー | 条件 |
|---|---|
| ReaderError | 不正なソース |
| NameError | 未定義の名前の参照 |
| DefinitionError | 不正な定義、再定義、未定義名へのset! |
| ArityError | 引数数の不一致 |
| TypeError | 型が一致しない操作、無効なMapキー |
| IndexError | Vector、String、Bytesの添字やsliceの範囲外アクセス |
| ArithmeticError | 整数オーバーフロー、ゼロ除算、非有限Float値を生む演算 |

エラーには可能な範囲でファイル名、1始まりの行と列、原因を含めます。
列はUnicodeコードポイント単位です。
関数本体で発生したエラーには、本体の定義元の位置を使用します。
