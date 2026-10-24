# 継続と複数値の制御機能

#51でVMの継続とResultsを、標準の制御手続きと複数値の構文へ接続しました。
内部のNewCore環境で利用でき、公開CLIへの切替は後続作業です。
標準importは[内部Runtime](https://github.com/0x6d61/guri/wiki/Libraries)へ接続済みです。
#52で[dynamic-wind](https://github.com/0x6d61/guri/wiki/Dynamic-Wind)の動的範囲の移動を接続しました。

## 手続きと構文の一覧

| 名前 | 記法と引数数 | 結果と共有 |
|---|---|---|
| call-with-current-continuation / call/cc | `(call/cc proc)`。1引数。procは1引数を受ける手続き | 戻り先を表す継続手続きをprocへ渡す。procの全結果を返す。継続は制御状態を保存し、Cellや格納された値は共有する |
| values | `(values value...)`。0引数以上、すべての型 | 全引数を複数値として返す。ListやVectorへ変換しない |
| call-with-values | `(call-with-values producer consumer)`。2引数 | producerを0引数で呼び、返った全値をconsumerの引数へ渡す。consumerの全結果を末尾転送する |
| apply | `(apply proc fixed... list)`。2引数以上 | 最後の正規Listの要素を固定引数の後へ連結し、procへ末尾転送する。Listを変更せず、要素は共有する |
| dynamic-wind | `(dynamic-wind before thunk after)`。3引数 | 通常の入退場と継続の脱出・再入でフックを呼び、thunkの全結果を返す。詳細は専用ページ |
| define-values | `(define-values formals expr)`。2引数 | exprの複数値を定義する。トップレベルでは再定義可能、内部では本体全体の束縛を作る。結果は未規定値 |
| let-values | `(let-values ((formals expr) ...) body...)` | 全exprを外側の環境で評価してから束縛し、本体の全結果を返す |
| let*-values | `(let*-values ((formals expr) ...) body...)` | 各exprを先行する束縛の環境で評価してから次の範囲を作り、本体の全結果を返す |

formalsはlambdaと同じく`(a b)`、`(a . rest)`、全値を受ける`rest`を使えます。
restは新しく構築したListで、格納する値を深くコピーしません。
同じformals内での名前の重複と、let-valuesの異なる束縛群にまたがる重複はエラーです。
let*-valuesでは後続の群で同じ名前を使い、新しい束縛で隠せます。

## call/ccと保存した継続

```scheme
(call/cc (lambda (exit) (exit 42))) ; => 42
```

継続は現在の計算を中断し、保存した戻り先へ渡された値を返します。
元の呼び出しが終了した後も、同じVMで繰り返し使えます。
引数の評価途中、ネイティブ手続きからの再開、残りのトップレベル式も保存対象です。
変数やPairなどの変更は巻き戻さず共有します。

call/ccとcall-with-current-continuationは同じ手続き値を参照します。
継続もSchemeの手続き値であり、applyや通常の手続き呼び出しで呼べます。
別のMachineに属する継続を持ち込む内部API操作は診断します。

## 複数値と値の要求数

```scheme
(call-with-values (lambda () (values 20 22))
                  (lambda (a b) b)) ; => 22
```

Resultsは0個以上の値を保持するVMの返却形式です。
通常のコレクション型とは区別します。
式の最後では全結果を維持し、beginなどの途中の式では任意個の結果を捨てます。
演算子、実引数、ifの条件などの単一値を要求する箇所では、値の個数を検査します。
producer内で保存した継続にはconsumerへ渡す処理も含み、再開時にも同じ個数の検査を適用します。

## apply

```scheme
(apply (lambda (x) x) '(42)) ; => 42
```

末尾引数は正規Listに限ります。
Vector、非正規List、循環Listは型エラーにし、無限に走査しません。
呼び出し先の引数数と型の検査は、通常の呼び出しと同じ経路を通ります。
従来の独自GuriのVector展開は、内部の標準applyへ持ち込みません。

## define-valuesと束縛の範囲

```scheme
(define-values (a b) (values 20 22))
b ; => 22
```

内部定義は他のdefineと同じ範囲へ束縛を確保します。
右辺の式の値の個数を検査してから保存し、初期化前の参照はエラーです。
末尾のrestや、0個の値を受ける空のformalsにも対応します。

```scheme
(let-values (((a b) (values 20 22))) b) ; => 22
(let*-values (((x) (values 42)) ((y) (values x))) y) ; => 42
```

let-valuesの初期値から、同じ群で新しく作る変数は見えません。
let*-valuesでは先行する束縛が見え、後続の束縛が先行クロージャへ遡って見えることはありません。
本体の最後は末尾位置を維持します。

## エラーと内部実装

引数数が合わない手続きやconsumerはArityErrorに対応するVMのエラーです。
単一値を要求する箇所での個数違い、applyの非List、非手続きの呼び出しも原因付きで診断します。
不正なformalsや重複はコンパイル時のSyntaxErrorです。
これらをGoから見る場合はerrors.Isで元のVMやSchemeの原因を調べられます。

制御手続きはVMのAction/Resume/Capture/Resultsを使います。
Schemeへの再呼び出しをGoの再帰へ隠さず、call/ccのproc、applyのproc、call-with-valuesのconsumerはInvokeで末尾転送します。
Resumeの制御状態は変更せず、保存された継続から何度でも再開できます。

複数値の構文は、保護した制御手続きとクロージャへloweringします。
let-valuesでは初期値の結果を一時的に保持してから実際の変数を作り、let*-valuesでは順番に範囲を作ります。
通常の変数でcall-with-valuesやapplyを隠しても、構文の動作を誤って変えません。
VMの命令や公開Go APIの契約は変更していません。

## 検証

TestStandardControlProcedures、TestMultipleValueBindingRegionsAndDefinitions、TestControlMultiShotAndProperTailCallsで手続きと構文を検証します。
TestMultiShotProducerContinuationAndCyclicApplyでは、producer内の複数回再開、失敗後の再利用、循環Listの拒否も確認します。
必須の末尾転送を経由した10万回の呼び出しでフレーム数が一定であることを検証します。
使用例はTestControlReferenceExamplesとExampleEngine_Eval_valuesで検証します。
標準ライブラリのimportと公開CLIによる結合テストの成功とは区別します。

参照は[R7RS §3.5](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-5.html#TAG:__tex2page_sec_3.5)、[§4.2.2](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-6.html#TAG:__tex2page_sec_4.2.2)、[§5.3.3](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-7.html#TAG:__tex2page_sec_5.3.3)、[§6.10](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.10)です。
