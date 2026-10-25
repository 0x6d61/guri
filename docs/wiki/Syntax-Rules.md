# syntax-rulesマクロ

Guriの内部R7RS環境に、衛生的なマクロを定義して展開する仕組みを追加しました。
このページでは#49で実装した構文と、#48のコンパイラへの接続を説明します。
公開CLIは従来の実装を使用し、標準のimportとライブラリの公開は#61で接続します。
内部環境の成功をR7RS全体の準拠完了とは扱いません。

## 使用例

`core.NewCore()`でマクロ構文も利用できます。
標準の算術などの手続きは自動では入りません。
次の例は新しい手続きを注入せず、コア構文だけで実行できます。

```go
e := core.NewCore()
result, err := e.Eval(context.Background(), "macro.scm", `
(define-syntax my-if
  (syntax-rules () ((_ test yes no) (if test yes no))))
(let ((if (lambda (a b c) c))) (my-if #t 42 0))`)
if err != nil { panic(err) }
value, err := result.Single()
if err != nil { panic(err) }
fmt.Println(value.(*scheme.Integer).String())
// 42
```

出力はExampleEngine_Eval_macrosで検証します。
利用側のifという変数は、マクロが挿入するif構文を隠しません。
この例のように意図しない名前の捕捉を防ぐ性質を**衛生性**と呼びます。
以下のScheme例の`; =>`は返り値を示し、コードブロックごとに独立したNewCore環境で実行します。

## 構文一覧

| 構文 | 引数と記法 | 結果と変更 |
|---|---|---|
| define-syntax | `(define-syntax keyword transformer)`。識別子とtransformerの2引数 | 展開環境へ構文束縛を定義する。実行時の値のCellを更新する操作ではない。トップレベルでの内部API上の結果は未規定値 |
| let-syntax | `(let-syntax ((keyword transformer) ...) body...)` | transformerは外側の文脈で解釈し、本体で新しい構文束縛を使う。本体の最後の全結果を返す |
| letrec-syntax | `(letrec-syntax ((keyword transformer) ...) body...)` | transformerと本体の両方に新しい構文束縛を見せる。相互参照でき、本体の最後の全結果を返す |
| syntax-rules | `(syntax-rules (literal ...) (pattern template) ...)`、独自ellipsisを指定する形 | transformerを構築する。実行時の通常の式や手続き値としては使えない |
| syntax-error | `(syntax-error message arg...)`。文字列リテラルと0個以上の補足引数 | 展開時にSyntaxErrorを返す。補足引数は評価しない |

本体には先頭の0個以上の定義と1個以上の式を置きます。
本体の最後の式は0個以上の返り値を維持し、末尾位置も引き継ぎます。
同じlet-syntaxまたはletrec-syntaxの束縛群でキーワードを重複指定できません。
`_`と`...`も補助構文束縛として初期環境へ登録します。

## define-syntaxと内部定義

define-syntaxはキーワードにtransformerを結び付けます。
transformerにはsyntax-rulesを使います。
トップレベルと本体先頭で定義でき、通常の変数で構文キーワードを隠すこともできます。

```scheme
(define-syntax answer
  (syntax-rules () ((_) 42)))
(answer) ; => 42
```

内部の構文定義は、その本体を範囲とする構文束縛を作ります。
マクロが展開して生成するdefine、define-syntax、定義を含むbeginも本体の定義として認識します。
式の後に現れる内部定義はエラーです。

```scheme
(define-syntax define-constant
  (syntax-rules () ((_ name value) (define name value))))
(define-constant answer 42)
answer ; => 42
```

パターンから受け取ったnameは利用側の文脈を保持するため、生成したanswerを後続の式から参照できます。
展開で導入した識別子は利用側の名前と区別し、SchemeのSymbolとしての綴りは変更しません。

## let-syntaxとletrec-syntax

let-syntaxのtransformerは外側の環境を参照します。
同じ束縛群にある別のキーワードの定義は、そのtransformerからは見えません。

```scheme
(let-syntax ((keep (syntax-rules () ((_ value) value))))
  (keep 42)) ; => 42
```

letrec-syntaxでは全キーワードを先に確保し、各transformerの定義からも参照できるようにします。
次の例ではmy-orが再帰的に展開されます。

```scheme
(letrec-syntax
    ((my-or
      (syntax-rules ()
        ((_) #f)
        ((_ first rest ...)
         (let ((saved first))
           (if saved saved (my-or rest ...)))))))
  (let ((saved 9))
    (my-or #f 42))) ; => 42
```

マクロが導入したsavedは、利用側のsavedと別の束縛です。
これらの構文はコンパイル時の環境を変更し、実行時には展開後のコア構文だけがVMへ渡ります。

## パターン照合

各ルールを記述順に照合し、最初に一致したルールを使います。
ルールの最初の要素は識別子を要求しますが、照合には使いません。
その後の識別子は、literal、ワイルドカード、ellipsis以外ならパターン変数です。

| パターン | 扱い |
|---|---|
| パターン変数 | 任意の構文に一致する。同じパターンでの重複はエラー |
| `_` | 任意の構文に一致し、値を束縛しない。複数回使える |
| literal識別子 | 定義元と利用元の束縛が同一なら一致する。両方未束縛なら同じ名前に一致する |
| 定数 | equal?に対応する値の一致を検査する。数値では正確性も区別する |
| Listと非正規List | 要素と末尾を照合する |
| Vector | Vectorの要素を照合する |
| `pattern ...` | 0個以上の要素を照合する。入れ子や、後続の固定パターンも扱う |

literalに挙げた`_`やellipsisは、ワイルドカードや繰り返しより優先してliteralとして照合します。
literal一覧で同じ識別子を複数回挙げても意味は変わりません。
数値の一致には採用済みの数値プロファイルを使い、1と1.0は正確性が異なるため区別します。
この比較の内部実装を、標準手続きequal?の公開完了とは扱いません。

## テンプレートと繰り返し

テンプレートのパターン変数を、照合した位置付きの構文へ置き換えます。
その他の識別子には展開ごとのスコープを付け、定義元の束縛を参照できるようにします。
ListとVectorの出力構造は新しく組み立て、置換した構文の文脈や値の共有を保持します。
文字列置換やSymbol名の書き換えでは処理しません。

パターンの繰り返しの深さを、テンプレートでも必要な数のellipsisで展開します。
入れ子の各回数を保持するため、空の行や長さの異なる行を含むデータも扱えます。
複数のパターン変数を同じ繰り返しで並べる場合は、その段階の長さが一致しなければエラーです。
繰り返しを進めるパターン変数がないテンプレートもエラーです。

独自のellipsis識別子を指定できます。

```scheme
(define-syntax seq
  (syntax-rules ::: ()
    ((_ first rest :::) (begin first rest :::))))
(seq 1 2 42) ; => 42
```

テンプレート中の`(... template)`は、その内側でellipsisを普通の識別子として扱うための記法です。
例えば`(... ...)`は1つのellipsisを出力します。
マクロから別のsyntax-rulesを生成するときにも使え、生成したマクロのliteral識別子の束縛も保持します。

## syntax-errorと診断

syntax-errorはメッセージに文字列リテラルを要求します。
引数は実行せず、診断用のdatumとして保持します。

```scheme
(define-syntax expect-one
  (syntax-rules ()
    ((_ x) x)
    ((_ rest ...) (syntax-error "expected one value" rest ...))))
(expect-one) ; => SyntaxError: expected one value
```

Goではcore.Errorが使用位置を持ち、原因のcore.SyntaxFailureがメッセージ、未評価のIrritants、定義位置を持ちます。
errors.Isでcore.ErrSyntaxを、errors.Asで診断情報を取得できます。
ルール不一致や繰り返しの長さ不一致も、可能な範囲で呼び出し位置とtransformerの定義位置を返します。
これらはコンパイル時の失敗なので、実行時の例外ハンドラで捕まえられることは保証しません。
Engine.Evalへ渡したcontextのキャンセルは、展開と照合でも確認します。

## 衛生性の内部設計

識別子は#45のSymbolとScopeの組を維持します。
Scopeには展開器の定義元文脈を保持できるようにし、通常のNewScopeの同一性の契約は変えません。
定義元文脈はレキシカルな範囲と、グローバル束縛の同一性を保持します。
内部定義の範囲は、本体の定義を発見する過程で完成させます。

解決時は、同じ文脈の導入済み束縛を先に探します。
なければ導入スコープの定義元文脈へ戻り、自由識別子の束縛を解決します。
局所変数では束縛オブジェクトの同一性からVM環境の深さとスロットを求めます。
利用元の同名変数へ置き換えて解決しないため、自由識別子の誤捕捉を防げます。
定義元の変数のCellは共有し、通常の代入による値の更新は反映します。

各パターン変数の照合結果は、繰り返しの各段階を保持する木へ格納します。
0回の繰り返しも明示し、テンプレートの同じ深さで対応する結果を取り出します。
展開結果は既存のコンパイラへ渡し、VM命令の契約は変更しません。

## 検証と未接続範囲

TestMacroHygieneAndDefinitionBindings、TestLiteralIdentifiersUseBindingIdentity、TestEllipsesVectorsAndDottedPatternsなどで、捕捉回避、束縛の一致、再帰、入れ子、Vector、非正規Listを検証します。
マクロを生成するマクロと、そのliteralの束縛は回帰テストで確認します。
使用例はTestMacroReferenceExamplesとExampleEngine_Eval_macrosで検証します。
従来の公開CLIによる独自17件とChibi由来62件の結合測定は、内部展開器の成功と分けて記録します。

16標準ライブラリのimportは内部Runtimeと公開CLIへ接続済みです。
参照は[R7RS §4.3](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-6.html#TAG:__tex2page_sec_4.3)、[§5.4](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-7.html#TAG:__tex2page_sec_5.4)です。

## 共有と循環を含む引用テンプレート

#58でテンプレートの構築をdatumの同一性、繰り返し位置とellipsisの有効状態ごとに保持します。
長さが変わる展開の後に参照を解決するため、共有tailと循環を引用データへ残せます。
入力から捕捉したNodeを変更せず、パターン変数の置換と文脈を保ちます。
例と検証範囲は[グラフの読み書き](https://github.com/0x6d61/guri/wiki/Graph-IO)に記載します。

## 結合監査での修正

#66で入れ子のsyntax-rulesのliteral分類を修正しました。
パターンがliteral一覧に属するかは識別子の構文上の文脈で判定し、入力との照合に使う束縛比較と分けます。
トップレベルのbeginへ導入する変数のセルも本体のコンパイル前に予約し、手続き、多値、Recordの前方参照を保持します。
利用者の同名の公開束縛とは共有しません。
[結合監査](https://github.com/0x6d61/guri/wiki/R7RS-Audit)に再現例と検証結果を記載します。
