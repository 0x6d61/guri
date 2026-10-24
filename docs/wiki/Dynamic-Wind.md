# dynamic-windと動的な実行範囲

#52で、VMの動的状態の連鎖に入退場の処理を接続しました。
内部NewCore環境でdynamic-windを利用できます。
公開CLIの切替は後続作業です。
標準importは[内部Runtime](https://github.com/0x6d61/guri/wiki/Libraries)へ接続済みです。

## 手続きの仕様

`(dynamic-wind before thunk after)`は3個の手続きを受け取ります。
いずれも0引数で呼びます。
beforeを実行してからthunkへ入り、thunkから通常に戻るとafterを実行します。
返すのはthunkの全結果で、beforeとafterが返す0個以上の結果は捨てます。
格納した手続きやCellを深くコピーせず、副作用は共有します。

```scheme
(define stage 'initial)
(define answer
  (dynamic-wind
    (lambda () (set! stage 'before))
    (lambda () stage)
    (lambda () (set! stage 'after))))
(values answer stage) ; => before と after の2値
```

例はExampleEngine_Eval_dynamicWindで検証します。
全引数が手続きでなければ型エラー、呼び出し時の引数数が合わなければVMの引数数エラーです。
内部標準環境の制御手続きの一覧は[継続と複数値](https://github.com/0x6d61/guri/wiki/Control-Features)を参照してください。

## 継続による脱出と再入

継続を呼んで範囲の外へ移ると、退出する範囲のafterを内側から順に呼びます。
保存した継続で範囲の中へ戻ると、入る範囲のbeforeを外側から順に呼びます。
元の範囲と移動先の共通部分では、beforeもafterも繰り返しません。
兄弟の範囲への移動では、退出側のafterを先に実行します。

beforeとafterは、対応するthunkの範囲の外で実行します。
入る場合はbeforeが正常に戻ってから範囲を有効にし、出る場合は範囲を外してからafterを呼びます。
そのため、beforeで外へ逃げた場合はthunkへ入らず、そのafterも実行しません。
afterで外へ逃げた場合も同じafterを重複実行しません。

元のdynamic-windの呼び出しが終わった後でも、継続から繰り返し再入できます。
結果の値を保ったままフックを実行し、変数の更新を巻き戻しません。

## VMの移動手順

Dynamicは同一性を持つ不変な親連鎖です。
Windの要素はbeforeとafterを保持し、その他の動的な要素はpayloadを保持できます。
移動元と移動先の共通の祖先を求め、退出または再入を1段階ずつ進めます。

Action.WithDynamicと継続のジャンプは、同じ移動処理を通ります。
フックもInvoke/Resumeで呼び、途中の処理を不変なActionとして保持します。
Goの関数を再帰呼び出しして、まだ実行していないフックをGoスタックへ隠すことはありません。
目的の範囲へ移動してから、継続のフレームと返り値を復元します。
通常の戻りも、thunkの全結果を保持してafterを呼び終えてから返します。

beforeやafter自身で保存した継続を再開する場合、この実装は中断時点の移動手順も再開します。
ただし、そのフック自身の動的範囲へ継続で出入りする効果はR7RSで未規定です。
テストした実装動作を、移植可能なSchemeプログラムへの保証とは扱いません。

## 例外とホストの中断の境界

Schemeのハンドラ、raise、パラメータは#53で同じDynamic連鎖へ接続しました。
現時点でGoのVM診断やcontextのキャンセルを、そのままSchemeのraiseとして扱いません。
従来のGoレベルの実行中断は呼び出し元へエラーを返す経路であり、明示的なraiseとは別の経路です。
未処理の明示的なraiseはWindを退出してからGoへ返します。
公開Go APIの例外境界やポートの資源管理は、それぞれの担当Issueで区別して扱います。

## 検証

TestWindNormalNestedAndEscapeOrderで通常の順序、入れ子、フックからの脱出を検証します。
TestWindMultiShotReentryAndSiblingTransferで終了後の複数回再入と兄弟間の順序、TestWindSharedPrefixAndMultipleResultsで共通範囲と複数値を検証します。
TestWindContinuationsInsideHooksではbefore/after内での保存と再開、手続きの型を確認します。
既存の末尾呼び出し、複数値、VMの動的状態の保存テストも維持します。

参照は[R7RS §6.10](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.10)です。
