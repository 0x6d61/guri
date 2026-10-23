# 例外と動的パラメータ

#53で例外ハンドラ、guard、error-object、パラメータを内部VMへ接続しました。
標準の例外と、規格が単にエラーであるとする操作のGo診断を区別します。
公開Go APIとCLIは引き続き従来の実装を使用します。

## 手続きと構文

| 名前 | 引数と結果 |
|---|---|
| with-exception-handler | `(with-exception-handler handler thunk)`。2個の手続き。handlerは1引数、thunkは0引数。thunkの全結果を返す |
| raise | `(raise object)`。任意の型の1引数をハンドラへ渡す。ハンドラが通常復帰すると親へ二次例外を送る |
| raise-continuable | `(raise-continuable object)`。任意の型の1引数。ハンドラの全結果で元の呼び出しを再開する |
| error | `(error message irritant...)`。文字列と0個以上の補足値。error-objectを作り非継続可能な例外を送る |
| error-object? | 任意の1引数。内部のerror-objectなら#t |
| error-object-message | error-objectの1引数。メッセージのStringを返す |
| error-object-irritants | error-objectの1引数。補足値のListを返す |
| read-error? / file-error? | 任意の1引数。対応するエラー分類なら#t、それ以外は#f |
| make-parameter | `(make-parameter initial [converter])`。1〜2引数。0引数で現在値を読むパラメータ手続きを生成する |
| parameterize | `(parameterize ((parameter value) ...) body...)`。変換した値を動的に束縛し、本体の全結果を返す |
| guard | `(guard (variable clause...) body...)`。条件をvariableへ束縛してcond形式の節を調べ、選んだ全結果を返す |

ハンドラ、例外値、パラメータの値は深くコピーしません。
errorは補足値を新しいListへ浅く格納し、メッセージをStringとして保持します。
ハンドラの追加とパラメータの変更は不変な動的連鎖へ記録し、通常の終了や継続の移動に合わせて復元します。

## 継続可能な例外と再送出

```scheme
(with-exception-handler
  (lambda (condition) 42)
  (lambda () (raise-continuable 'problem))) ; => 42
```

ハンドラを実行するときは、現在のハンドラだけを、そのハンドラの設置時に有効だった親へ切り替えます。
raise地点のWindとパラメータは維持します。
そのためハンドラから再びraiseすると親が呼ばれ、同じハンドラへ無限に再送出しません。
raise-continuableから通常に戻る場合は、元のハンドラを復元します。

raiseのハンドラが戻った場合、通常の復帰値として採用しません。
親へ送る二次例外はerror-objectとし、元の条件を補足値へ含めます。
規格は二次例外の条件との関係を未規定としているため、この補足情報を移植可能な仕様とは扱いません。

## guardと動的な範囲

```scheme
(guard (condition (else condition))
  (raise 42)) ; => 42

(guard (condition ((error-object? condition)
                   (error-object-message condition)))
  (error "bad input" 1 2)) ; => "bad input"
```

guardの節は、guard式の継続と動的環境で評価します。
元のraise地点の内側にWindがあれば、そのafterを実行してから節へ移ります。
一致する節がない場合はraise地点へ戻り、親のハンドラへ継続可能な例外として再送出します。
その移動でもWindのbeforeとafterを実行します。
通常に本体が終了した場合も、0個以上の結果を維持します。
guardの変数がelseや=>という名前の場合も、そのレキシカルな束縛を優先します。

## パラメータと変換

```scheme
(define p (make-parameter 1))
(parameterize ((p 42)) (p)) ; => 42
(p) ; => 1
```

make-parameterのconverterを指定した場合、初期値にも変換を適用します。
parameterizeでは、parameter式とvalue式を外側の動的環境で評価し、各値を変換してから新しい束縛群を有効にします。
そのconverterも外側の動的環境で実行します。
値は任意の型を扱い、converterの結果は1個でなければエラーです。
標準で要求される0引数の参照を提供し、パラメータ手続きへの追加引数による独自のsetterは提供しません。

保存した継続へ戻ると、その時点の変換済みの束縛を復元します。
converterを再実行することはありません。
converter内で保存した継続を再開する場合も、途中の束縛作成を不変な状態として保持します。
パラメータを装った通常の手続きをparameterizeへ渡すことはできません。

## 内部の表現とGoへの境界

ハンドラの選択変更は、現在のDynamic連鎖の上に置く不変な選択情報です。
Windやパラメータを外す操作とは分けるため、ハンドラを呼ぶ前に誤ってWindを退出しません。
パラメータはVMのCallable契約を実装する手続きで、動的束縛はパラメータの同一性から値への不変な対応です。
Schemeから見る独自データ型は追加しません。

error-objectは既存のRecordを非公開の型識別で使います。
利用者が同じ名前のレコード型を作ってもerror-objectにはなりません。
内部のNewErrorObjectとRaiseは、後続のread/fileプリミティブが分類付きの例外を送るための接続先です。
read/fileの入出力手続きそのものは担当Issueで実装します。

未処理の明示的なraise/errorは、Windを退出してから既存のscheme.RaisedをGoへ返します。
Goではerrors.Asで条件値、継続可能性、位置を取得できます。
error-objectのメッセージはGoの診断にも含め、afterの実行後も元の発生位置を保持します。
従来の内部VM診断やcontextのキャンセルをすべてSchemeのraiseへ変換することはしていません。
規格が例外を要求する後続のプリミティブは、明示的にこの例外経路へ接続します。
コンパイル時のReader/SyntaxErrorを実行時のguardで捕まえられる保証もありません。

## 検証

TestExceptionHandlersAndGuard、TestExceptionsPreserveWindAndDynamicParametersで、通常復帰、二次例外、再送出、Windとの順序、複数値を検証します。
TestParametersConversionRegionAndReentryとTestErrorCategoriesAndConverterContinuationで、変換、範囲、継続再入、エラー分類を検証します。
guardの補助構文のシャドーイング、未処理エラーのメッセージと位置も回帰テストで確認します。
このページのScheme例はTestExceptionReferenceExamplesで実行します。

参照は[R7RS §4.2.6、§4.2.7](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-6.html)、[§6.11](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.11)です。
