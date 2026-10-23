# Schemeのコア構文とコンパイラ

#48で、新Readerの構文をバイトコードへコンパイルし、#47のVMで実行する`internal/core`を追加しました。
このページは内部のR7RS移行環境のリファレンスです。
公開CLIと従来のInterpreter.Evalは引き続き旧実装を使用します。
[従来の特殊形式](https://github.com/0x6d61/guri/wiki/Special-Forms)と、このページの構文を区別してください。

## 初期環境と使用例

`core.NewCore()`は以下の10個の構文束縛を持つ内部環境を作ります。
標準ライブラリをimportした環境ではなく、算術、println、call/ccなどの手続きは自動で入りません。
標準のimportとライブラリ環境は#61、衛生的なマクロ展開は#49で接続します。
テストや将来のライブラリ実装では、Engine.Bindで手続き値を明示的に与えられます。
テストで注入する単純な整数用の演算を、標準の数値手続きの実装とは扱いません。

```go
e := core.NewCore()
results, err := e.Eval(context.Background(), "example.scm",
    `(define (choose x) (if x 42 0)) (choose '())`)
if err != nil { panic(err) }
value, err := results.Single()
if err != nil { panic(err) }
fmt.Println(value.(*scheme.Integer).String())
// 42
```

この出力はinternal/core/example_test.goで検証します。
以下のScheme例の`; =>`は返り値を示し、標準出力への表示ではありません。
コードブロックごとに独立したNewCore環境で実行します。
TestReferenceExamplesWithoutInjectedProceduresなどで例を検証します。

## 構文一覧

| 構文束縛 | 記法と引数 | 結果と変更 |
|---|---|---|
| quote | `(quote datum)`、`'datum`。1引数 | datumを評価せず返す。定数として変更禁止 |
| lambda | `(lambda formals body...)`。formalsと1つ以上の本体式。先頭に内部定義を置ける | 環境を保持する新しい手続き |
| if | `(if test yes [no])`。2〜3引数 | 選んだ分岐の全結果。no省略で偽なら未規定値 |
| set! | `(set! variable expr)`。2引数 | 既存のCellを更新し、未規定値 |
| define | `(define variable expr)`、`(define (name . formals) body...)` | トップレベルの定義・再定義、または本体のローカル定義。未規定値 |
| begin | `(begin expr...)`。式の位置では1式以上 | 順次評価し、最後の全結果。トップレベル・本体先頭では定義をまとめられる |
| let | `(let ((name init) ...) body...)`、named let | 外側で初期値を評価し、新しいCellへ束縛。本体の全結果 |
| let* | `(let* ((name init) ...) body...)` | 各初期値の後に新しい束縛範囲を作る。本体の全結果 |
| letrec | `(letrec ((name init) ...) body...)` | 全Cellを先に確保し、全初期値を評価後に保存。本体の全結果 |
| letrec* | `(letrec* ((name init) ...) body...)` | 全Cellを先に確保し、初期値の評価と保存を順に行う。本体の全結果 |

本体は0個以上の内部定義と1個以上の式です。
let*以外では、同じ束縛群の名前を重複指定できません。
式と実引数の型に一般的な制限はありませんが、呼び出す値は手続きでなければなりません。
演算子、各実引数、ifの条件、定義・代入の右辺、束縛の初期値は単一値を要求します。
本体やbeginの途中の式の結果は捨て、最後の式では0個以上の結果をそのまま返します。
空のプログラムと空のトップレベルbeginの内部API上の結果は未規定値です。

## リテラル、quoteと真偽

数値、Boolean、Character、String、Vector、Bytevectorは自己評価します。
Vector内のSymbolも変数として評価しません。
quoteは任意のdatumを返し、Listを含めて値を定数として扱います。

```scheme
'symbol ; => symbol
(if '() 1 2) ; => 1
```

偽値は#fだけです。
空Listは真値ですが、裸の`()`を式として書くことはできません。
`nil`、`true`、`false`、`def`、`defn`、`fn`は特別扱いせず、束縛がなければエラーです。

リテラルは共有する内部のPair、String、Vector、Bytevectorまで変更禁止にします。
Readerが通常のデータとして返す値は変更可能なままで、プログラムの定数としてコンパイルする際に凍結します。

## lambdaと手続き呼び出し

formalsには`(a b)`、`(a b . rest)`、全引数を受ける`rest`を使います。
引数名は識別子で、重複はエラーです。
restは呼び出しごとに新しいListを作り、余った引数の値を共有して格納します。
Vectorへまとめません。

```scheme
((lambda (x) x) 42) ; => 42
((lambda args args) 1 2) ; => (1 2)
((lambda (x . args) args) 0 1 2) ; => (1 2)
```

手続きは作成時のレキシカル環境を保持し、呼び出しごとに引数用の新しいCellを作ります。
同じ外側の変数を捕捉したクロージャはCellを共有します。
引数や変数へ渡したPairやVectorなどの値は深くコピーしません。
手続き呼び出しは演算子から実引数へ左から右に評価します。
これはR7RSが許す逐次順序の一つです。

## if、set!、define、begin

ifは条件を評価して片方の式だけを実行します。
最後の式や分岐が返す複数値も維持します。

```scheme
(define x 1)
(set! x 2)
x ; => 2
```

set!は既存の最も近い束縛セルを更新します。
未定義・未初期化の変数や構文束縛へ代入できません。
トップレベルのdefineは同じCellを再利用して再定義し、既に生成したクロージャからも新しい値が見えます。
内部のdefineは本体全体を範囲とする新しいローカル束縛です。

```scheme
(define (id x) x)
(id 42) ; => 42
```

手続き定義の略記は、コンパイラが直接クロージャ生成へ変換します。
本体は呼び出すまで実行しません。
beginは順次評価し、トップレベルや本体先頭では定義を含む列をまとめられます。

```scheme
(begin 1 2) ; => 2
```

## letとlet*

letは全初期値を外側の環境で評価します。
その後に新しいCellへ値を格納して本体を評価します。

```scheme
(let ((x 1) (y 2)) y) ; => 2
```

let*は先行する束縛を後続の初期値から参照できます。
同じ名前の再使用も別の束縛を作ります。

```scheme
(let* ((x 1) (y x)) y) ; => 1
```

各束縛の範囲を入れ子のクロージャ呼び出しへ変換するため、後続の束縛が先行クロージャへ遡って見えることはありません。
この範囲はTestCoreSyntaxAndLexicalClosuresで検証します。

## letrec、letrec*と内部定義

letrecは全変数のCellを未初期化で先に確保し、その範囲で全初期値を評価します。
全初期値が揃ってから保存するため、初期値から他の変数の値を先に読むことはできません。
クロージャへCellを捕捉させることで相互再帰を構成できます。

```scheme
(letrec ((first (lambda () (second)))
         (second (lambda () 42)))
  (first)) ; => 42
```

letrec*は同じ範囲に全Cellを作り、評価した値をその都度保存します。

```scheme
(letrec* ((x 1) (y x)) y) ; => 1
```

内部定義はletrec*と同じ意味を持つ内側の範囲へ変換します。
この範囲はlambdaの仮引数の範囲より内側なので、仮引数と同じ名前を定義して隠せます。
同じ本体の定義名の重複、初期化前の参照や代入、式の後に現れる内部定義はエラーです。
定義の範囲を発見するために使ったdefineやbeginの意味を、その定義自体で変える場合も診断します。

## named letと末尾位置

named letは本体内で名前付きの再帰手続きを参照できます。
初期値はその再帰名の範囲の外で評価します。

```scheme
(let loop ((again #t))
  (if again (loop #f) 42)) ; => 42
```

コンパイラは本体・beginの最後、ifの選択される分岐、各let形式の本体へ末尾位置を引き継ぎます。
末尾位置の呼び出しはTailCallを生成します。
演算子・実引数・初期値・set!/defineの右辺は末尾位置にしません。
テストではソースからコンパイルした10万回のnamed letでフレーム数が一定であることを確認します。

## 束縛解決とVMへの変換

構文キーワードも初期環境の束縛として登録します。
ソースの名前文字列を見てifやlambdaの処理を先取りせず、識別子の文脈と内側の束縛を解決してから構文か変数かを判定します。
ローカル変数のifやquoteは普通の手続きとして呼び出せます。
現時点で文脈の同一性を保持して解決し、#49で衛生的な導入・参照の規則へ接続します。

変数参照はレキシカルな深さとスロット、または解決済みのグローバルCellを持つ命令になります。
let類と内部定義はクロージャ・呼び出し・Cellの初期化へ変換します。
letrec用にVMのPop命令を追加し、全初期値を評価して一時保存した後に各Cellへ移します。
実行時に構文木をたどりません。

## エラーと残る機能

構文の形や引数数、識別子の重複、不正な定義位置は位置付きのcore.Errorとして返します。
実行時の未初期化参照、手続きの引数数、型、単一値の要求は原因を保持するvm.Errorになります。
未知の変数には将来のトップレベル定義と共有する未初期化Cellを作るため、その参照も未初期化束縛として診断します。
公開CLI向けのエラー表示の移行は後続作業です。

標準のimport、syntax-rules、define-syntax、define-values、残りの派生構文、標準手続きは未実装です。
既存の17件とChibi由来62件はimportと標準ライブラリを含む結合テストで、内部コンパイラのunitテストとは別に未対応を記録します。
R7RS全体の準拠済みとは扱いません。

参照は[R7RS §3.1、§3.4、§3.5](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-5.html)、[§4.1、§4.2.2〜4.2.4](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-6.html)、[§5.3](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-7.html)です。
