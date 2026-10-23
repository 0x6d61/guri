# Guri

GuriはLispのS式を使う、動的かつ強い型付けの言語です。
Goで実装したASTインタプリタが、シングルスレッドでプログラムを評価します。

## 言語仕様と関数リファレンス

- [言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification)
- [特殊形式](https://github.com/0x6d61/guri/wiki/Special-Forms)
- [全組み込み関数のリファレンス](https://github.com/0x6d61/guri/wiki/Functions)

原稿は [docs/wiki](docs/wiki) で管理します。
仕様や関数を変更するときは同じPRで原稿を更新し、マージ後に [Wiki公開手順](docs/WIKI.md) に従って同期します。

## ファイルの実行

Go 1.26以降を使用します。
リポジトリのルートで次のコマンドを実行すると、カウンターが `1`、`2` の順に出力されます。

```sh
go run ./cmd/guri examples/counter.guri
```

実行ファイルを作る場合は、次のコマンドを使います。

```sh
go build -o guri ./cmd/guri
./guri examples/counter.guri
```

CLIはファイルを1つ受け取り、トップレベルの式を上から順に評価します。
出力には `println` を使います。
評価エラーは標準エラー出力にファイル名、行、列と原因を表示し、終了コード1で終了します。
引数の使い方が不正な場合の終了コードは2です。

## 関数とデータ

```clojure
(defn make-adder [n] (fn [x] (+ n x)))
(def add10 (make-adder 10))
(println (add10 5)) ; 15

(def user {:name "Alice" :age 25})
(assoc! user :age 26)
(println (:age user)) ; 26

(def a [1 2 3])
(def b a)
(assoc! b 0 9)
(println a) ; [9 2 3]
```

`(defn name [args...] body...)` は `(def name (fn [args...] body...))` の糖衣構文です。
`defn` はトップレベルで関数を定義し、定義した関数を返します。
同じ名前の再定義はエラーになるため、値の再代入には `set!` を使います。

偽値は `nil` と `false` だけです。
`and` と `or` は短絡評価します。
`fn` はレキシカルスコープを持ち、外側の束縛セルを共有します。
可変長引数は `[x & rest]` と書き、残りの引数をVectorとして受け取ります。

反復処理は `(while condition body...)` と書き、終了時にnilを返します。
条件を反復ごとに評価し、本体は現在の環境で実行します。

```clojure
(let [n 0]
  (while (< n 3)
    (println n)
    (set! n (inc n))))
```

複数条件の分岐は `(cond condition expression ...)` と書きます。
最初の真値に対応する式だけを評価し、一致しなければnilを返します。

```clojure
(defn direction [command]
  (cond
    (= command ">") 1
    (= command "<") -1
    true 0))
(println (direction ">")) ; 1
```

[括弧対応表を作るサンプル](examples/search-brackets.guri) では、Stringのnth、while、condを組み合わせています。

Listの連結構造は不変です。
VectorはGoのSliceと同じく、代入時にヘッダをコピーして基底配列を共有します。
`append` の返り値を変数に保持するには、`(set! b (append b 4))` と書きます。
Mapは参照を共有し、`assoc`、`dissoc`、`update` は浅いコピーを、末尾に `!` がある操作は元のMapを変更します。

## 組み込み関数の引数

| 関数 | 引数 |
|---|---|
| `+`、`*` | 0個以上。空ならそれぞれ0、1 |
| `-` | 1個以上。1個なら符号反転 |
| `/` | 2個以上。常にFloatを返す |
| `=`、`<`、`<=`、`>`、`>=` | 2個以上 |
| `!=` | 2個 |
| `inc`、`not`、`len`、`cap`、`clone` | 1個 |
| `get` | Map、キー、省略可能なデフォルト値 |
| `assoc`、`assoc!` | MapまたはVector、キーまたは添字、値 |
| `dissoc`、`dissoc!` | Map、1個以上のキー |
| `update`、`update!` | Map、キー、関数、その関数への追加引数 |
| `append`、`conj` | Vector、0個以上の追加要素 |
| `nth` | Vector、StringまたはBytes、0始まりの添字 |
| `slice` | Vector、StringまたはBytes、開始位置、終了位置 |
| `reduce` | 関数、初期値、Vector |
| `println` | 0個以上。表示後にnilを返す |
| `error` | Stringのメッセージ1個。UserErrorで評価を中断 |

`=` はIntegerとFloatを数値として比較します。
Mapのキーの型はGoのMapと同様に区別するため、キーの `1` と `1.0` は別のキーになります。
Mapの列挙順序と表示順序は保証しません。
Stringの `len`、`nth`、`slice` はUnicodeコードポイント単位です。
不変のバイト列は `b"..."` と書き、バイト単位で扱います。
Bytesの `nth` は0〜255のIntegerを返し、`slice` はBytesを返します。
従来のStringの `len` がバイト長だった仕様から変更しています。

```clojure
(println (len "日本語") (len b"日本語")) ; 3 9
(println (nth "日本語" 1)) ; 本
(println (slice "Alice" 0 3)) ; Ali
(println (nth b"Alice" 0)) ; 65
```

## エラーによる中断

v0.0.1では `(error message)` で、指定したStringをメッセージとするUserErrorを発生させます。
関数やループからも評価を中断でき、CLIは診断を表示して終了コード1で終了します。
Go APIではerrorが返り、ホストプロセスは終了しません。

```clojure
(defn positive [n]
  (if (<= n 0) (error "n must be positive") n))
(println (positive 3)) ; 3
```

## 末尾呼び出し

v0.0.2向けに末尾呼び出しを実装しています。
自己再帰、相互再帰とクロージャの末尾呼び出しを、回数に比例してGoのスタックを増やさず実行します。

```clojure
(defn sum [n acc]
  (if (= n 0) acc (sum (- n 1) (+ acc n))))
(println (sum 100000 0)) ; 5000050000
```

apply、return、break、exitは引き続きv0.0.2の計画です。
詳細は [バージョン計画](https://github.com/0x6d61/guri/wiki/Roadmap) を参照してください。

## Goからの利用

```go
interpreter := guri.NewInterpreter()
value, err := interpreter.Eval("example.guri", "(+ 1 2)")
```

`Eval` は最後の式の値を返し、空入力ではnilを返します。
同じInterpreterに対する呼び出しはトップレベルの定義を保持します。
`interpreter.Output` に `io.Writer` を設定すると、`println` の出力先を変更できます。
Interpreterは複数のgoroutineから同時に使用できません。

## 開発時の検証

```sh
go test ./...
go vet ./...
```

開発手順は [AGENTS.md](AGENTS.md) に従います。
