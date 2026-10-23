# 派生構文と準引用

#50で、内部R7RSコンパイラに条件分岐、反復、case-lambda、準引用を接続しました。
構文束縛の解決後にコア構文へ変換し、VMで実行します。
公開CLIの切替と標準ライブラリの公開は後続作業です。

## 構文一覧

| 構文 | 記法と引数 | 結果と変更 |
|---|---|---|
| cond | `(cond clause...)`。節は`(test expr...)`、`(test => recipient)`、最後の`(else expr...)` | 最初に真になる節を評価。testだけならその値。recipientへはtestの値を1回渡す。該当なしは未規定値 |
| case | `(case key clause...)`。節の先頭はdatumのList、最後にelseを置ける。`=>`にも対応 | keyを1回評価しeqv?に対応する比較で選択。recipientにはkeyを渡す。該当なしは未規定値 |
| and | `(and expr...)` | 左から評価し、#fで短絡。最後の全結果。空なら#t |
| or | `(or expr...)` | 最初の真値で短絡。最後の全結果。空なら#f |
| when / unless | `(when test expr...)`、`(unless test expr...)`。本体は1式以上 | 真／偽の場合だけ本体を順に評価。結果は未規定で、現在は選択した本体の全結果または未規定値を返す |
| do | `(do ((var init [step]) ...) (test result...) command...)` | 初期値を外側で評価し、各stepの値を同時に新しい束縛へ渡す。終了時はresultの全結果。step省略は現在値 |
| case-lambda | `(case-lambda (formals body...) ...)` | 引数数が合う最初の節を使う手続きを生成。固定、dotted rest、全引数restを扱い、restはList |
| quasiquote | `(quasiquote template)`、バッククォート略記 | 静的な部分は定数、動的な部分は新しいPairやVectorとして構築 |
| unquote / unquote-splicing | 準引用内の`(unquote expr)`と`(unquote-splicing expr)`、コンマ略記 | 適切な入れ子の深さで評価し、splicingは要素位置へ正規Listを展開 |

elseと=>は補助構文として束縛します。
同名のローカル変数で隠した場合は、cond/caseの補助構文として先取りしません。
定義と式の範囲はコア構文と同じで、これらの式の本体に任意の位置でdefineを書けるようにはしません。

## 評価回数と末尾位置

condのtestとcaseのkeyは1回だけ評価します。
=>のrecipientは、選択された場合にだけ評価して呼び出します。
選択した節の最後、and/orの最後、when/unlessの本体最後、doの終了結果、case-lambdaの本体に末尾位置を引き継ぎます。
条件やrecipientそのものの評価は末尾位置にしません。

```scheme
(cond ((+ 1 2) => (lambda (x) (+ x 1))) (else 0)) ; => 4
(case 42 ((1) 0) (else => (lambda (x) x))) ; => 42
(or #f 42 missing) ; => 42
```

これらはTestDerivedConditionalsAndEvaluationCountsで検証します。
例の算術はテスト側で明示的に注入する単純な演算です。
標準の数値手続きの公開完了とは扱いません。

## 反復と手続きの節

```scheme
(do ((i 0 (+ i 1)) (sum 0 (+ sum i)))
    ((= i 5) sum)) ; => 10

((case-lambda (() 0) ((x) x) ((x y) (+ x y))) 42) ; => 42
```

doの初期値とstepは左から右に評価しますが、全stepを評価してから次の束縛へ渡します。
名前は重複指定できません。
case-lambdaは合う節がなければ引数数エラーです。
節ごとのクロージャを保持し、VMのInvokeで選択先へ末尾転送します。
名前付きletは#48で実装済みです。
標準doを旧Guriの逐次実行形式として扱わず、whileやreturn/breakも内部標準環境へ追加しません。

## 準引用と共有

```scheme
`(a ,(+ 1 2) ,@'(4 5) . 6) ; => (a 3 4 5 . 6)
`#(a ,(+ 1 2) ,@'(4 5)) ; => #(a 3 4 5)
```

入れ子の準引用では深さを増やし、unquoteで減らします。
対応する深さに達した式だけを実行します。
非正規Listの末尾にもunquoteを使えますが、その位置にunquote-splicingは使えません。
splicingの結果は正規Listでなければ型エラーです。

静的な部分は定数として再利用し、変更を禁止します。
実行時に構築するPairとVectorは可変です。
格納する値は深くコピーせず、splicingでは入力Listの要素を浅くコピーして接続します。
空のsplicing、Vector、入れ子、静的な部分の変更禁止をTestQuasiquoteTemplatesAndSharingで検証します。
特殊な準引用のキーワードを誤った位置や引数数で使う場合は構文エラーです。

## includeとfeatureの内部契約

| 構文 | 契約 |
|---|---|
| include / include-ci | 1個以上の文字列リテラルを受け、指定順でソースを読み、beginへ置き換える。include-ciはファイルごとの初期case foldingを有効にする |
| cond-expand | featureの識別子、and/or/not、library requirementを静的に照合し、選んだ節だけを展開する。通常の変数の値には依存しない |

Engine.SetSourceLoaderへ、context、包含元のファイル名、要求名を受ける関数を注入します。
返されたファイル名は診断に使い、元の行と列を維持します。
Engine.SetFeaturesは有効なfeature名と、library nameを照合する関数を受けます。
テストはメモリ内のソースで順序、内部定義、fold-case、未選択節を展開しないことを確認します。

NewCoreは実ファイルの検索ポリシーや標準ライブラリを設定しません。
loader未設定のincludeはErrSourceLoaderを返します。
検索パス、実ファイルの配布、公開するfeatureとlibrary一覧は#61で接続します。
未実装のライブラリを利用可能なfeatureとして宣言しません。

## エラーと内部設計

不正な節、elseの位置、=>の引数数、重複するcase datum、doの変数名、誤った準引用は位置付きのcore.Errorです。
手続きの引数数、splicingの型などはVMのエラーとして返します。
includeの読取エラーは包含先の位置、loaderの失敗は包含を要求した位置を保持します。

loweringで使うifやletなどは初期の構文束縛を保持し、利用側の再定義に捕捉させません。
一時変数は新しい導入スコープを持ちます。
準引用の構築やcase-lambdaの分岐には非公開のVMネイティブ補助処理を使い、不完全な標準手続き名を公開しません。
実行中にASTをたどることはありません。

参照は[R7RS §3.5](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-5.html#TAG:__tex2page_sec_3.5)、[§4.1.7、§4.2.1、§4.2.4、§4.2.8、§4.2.9](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-6.html)です。
