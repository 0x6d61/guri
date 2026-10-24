# 特殊形式

このページは公開CLIの従来の独自構文です。
移行中の内部環境のコア10形式は[Schemeのコア構文](https://github.com/0x6d61/guri/wiki/Core-Syntax)、マクロの5形式と補助構文は[syntax-rulesマクロ](https://github.com/0x6d61/guri/wiki/Syntax-Rules)に記載します。

特殊形式は、引数を評価するかどうかや評価する環境を制御します。
通常の関数のようにすべての引数を先に評価するわけではありません。
このページでは現在の全12形式を説明します。
構文の `...` は繰り返しを示し、実際のソースには書きません。

| 形式 | 構文 | 返り値 |
|---|---|---|
| def | `(def name expr)` | 定義した値 |
| defn | `(defn name [args...] body...)` | 定義したFunction |
| set! | `(set! name expr)` | 再代入した値 |
| fn | `(fn [args...] body...)` | Function |
| let | `(let [name expr ...] body...)` | 本体の最後の値。空ならnil |
| if | `(if cond yes)` または `(if cond yes no)` | 選んだ式の値。no省略で偽ならnil |
| and | `(and expr...)` | 最初の偽値、または最後の値。空ならtrue |
| or | `(or expr...)` | 最初の真値、または最後の偽値。空ならnil |
| do | `(do expr...)` | 最後の値。空ならnil |
| while | `(while condition body...)` | 終了時にnil |
| cond | `(cond condition expression ...)` | 最初の真値に対応する式の値。一致なしや空ならnil |
| quote | `(quote expr)` または `'expr` | 評価前のデータ |

引数数が不正な場合は `ArityError` になります。
評価した式のエラーは、そのまま呼び出し元に返します。

## def

トップレベルで名前を定義します。
nameはSymbolで、exprだけを評価します。
同じスコープで既存の名前を再定義した場合や、ローカル環境で定義した場合は `DefinitionError` になります。
組み込み関数の名前も既存の束縛として扱います。

```clojure
(def answer 42)
(println answer) ; => 42
```

## defn

`(def name (fn [args...] body...))` の糖衣構文です。
トップレベルで名前付き関数を定義し、その関数を返します。
defとfnと同じ制限を持ち、再帰、クロージャ、可変長引数に対応します。
本体は呼び出すまで評価しません。

```clojure
(defn fact [n]
  (if (= n 0) 1 (* n (fact (- n 1)))))
(println (fact 5)) ; => 120
```

## set!

最も近い環境にある既存の束縛を更新します。
名前自体は評価せず、exprを評価した結果を束縛セルに保存して返します。
未定義の名前やSymbol以外の名前は `DefinitionError` になります。
共有する束縛セルの更新は、そのセルを参照するクロージャにも見えます。

```clojure
(def n 1)
(println (set! n 2)) ; => 2
(println n) ; => 2
```

## fn

定義時の環境を保持したFunctionを返します。
引数リストはVectorで指定し、引数名は重複しないSymbolにします。
可変長引数は最後に `& rest` と記述します。
引数リストが不正な場合は `DefinitionError`、呼び出し時の引数数が合わなければ `ArityError` です。
本体は順に評価して最後の値を返し、空ならnilを返します。
本体の最後の式からの末尾呼び出しは、再帰回数に比例してGoのスタックを増やしません。
末尾位置のif、cond、do、let、and、orにもこの性質を引き継ぎます。

```clojure
(def collect (fn [first & rest] rest))
(println (collect 1 2 3)) ; => [2 3]
(println ((fn []))) ; => nil
```

## let

nameとexprを交互に並べたVectorで、ローカルな束縛を作ります。
各exprを順に評価するため、後続のexprから先行する束縛を参照できます。
束縛Vectorが不正な場合、名前がSymbol以外の場合、同じ名前を重複指定した場合は `DefinitionError` です。

```clojure
(println (let [x 2 y (+ x 3)] (* x y))) ; => 10
```

## if

condを評価して、真ならyes、偽ならnoだけを評価します。
noを省略してcondが偽ならnilを返します。
偽値はnilとfalseだけです。

```clojure
(println (if 0 "yes" "no")) ; => yes
(println (if false (/ 1 0) "safe")) ; => safe
(println (if nil 1)) ; => nil
```

## and

左から評価し、最初の偽値を返した時点で停止します。
すべて真値なら最後の値を返し、空ならtrueを返します。
返り値はBooleanに変換しません。

```clojure
(println (and 1 "ok")) ; => ok
(println (and false (/ 1 0))) ; => false
(println (and)) ; => true
```

## or

左から評価し、最初の真値を返した時点で停止します。
すべて偽値なら最後の値を返し、空ならnilを返します。
返り値はBooleanに変換しません。

```clojure
(println (or nil false "ok" (/ 1 0))) ; => ok
(println (or nil false)) ; => false
(println (or)) ; => nil
```

## do

同じ環境で式を順に評価して、最後の値を返します。
新しいローカルスコープは作りません。

```clojure
(def n 0)
(println (do (set! n 1) (inc n))) ; => 2
(println (do)) ; => nil
```

## while

各反復の前にconditionを評価し、真値の間は本体を左から順に実行します。
conditionがnilまたはfalseになったら終了してnilを返します。
本体の最後の値は返しません。
条件は必須で、省略すると `ArityError` です。
本体は0式以上を指定できます。

新しいスコープを作らず、既存の束縛セルやコレクションの更新を共有します。
条件や本体のエラーは、発生した式の位置を保って伝播します。
ループは再帰呼び出しで実装していないため、反復回数に比例してスタックを増やしません。
breakとcontinueは未対応です。

```clojure
(defn sum-to [limit]
  (let [n 0 total 0]
    (while (< n limit)
      (set! n (inc n))
      (set! total (+ total n)))
    total))
(println (sum-to 4)) ; => 10
(println (while false 1)) ; => nil
```

## cond

条件と式を交互に並べます。
条件を左から順に評価し、最初の真値に対応する式だけを評価して返します。
その後の条件や式は評価しません。
空の場合やすべての条件が偽値の場合はnilを返します。
デフォルト分岐は最後に `true` を条件として書けます。

条件と式が対にならない奇数引数は、条件を評価する前に `ArityError` になります。
新しいスコープは作らず、条件や選択した式のエラーは元の位置を保って伝播します。
1分岐で複数の式を実行するにはdoを使用します。

```clojure
(defn classify [command]
  (cond
    (= command ">") :right
    (= command "<") :left
    true :other))
(println (classify ">")) ; => :right
(println (classify "+")) ; => :other
(println (cond false (/ 1 0) true "safe")) ; => safe
(println (cond)) ; => nil
```

## quote

式を評価せず、データとして返します。
Symbolを環境から検索せず、Listを関数呼び出しとして扱いません。
VectorとMapの内部も評価しません。
引用したMapでも、キーの型の制限は適用されます。

```clojure
(println '(name 1 2)) ; => (name 1 2)
(println (quote [name])) ; => [name]
(println '{:name Alice}) ; => {:name Alice}
```

- [Promiseと遅延評価](https://github.com/0x6d61/guri/wiki/Lazy-Evaluation): scheme lazyの5束縛、メモ化と再入、force時の動的環境、delay-forceの空間要件。

- [環境、eval、loadとREPL](https://github.com/0x6d61/guri/wiki/Evaluation-REPL): datumの評価と内部REPLの環境。
