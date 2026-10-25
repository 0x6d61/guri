# R7RS-smallの結合監査

#66で内部Runtimeの結合テストを拡大し、マクロの不具合2種類を修正しました。
この監査結果は準拠宣言やリリース判定ではありません。
公開CLIは旧Guriのままであり、移行は#67で行います。

## 測定結果

| 測定 | 結果 | 範囲 |
|---|---|---|
| 内部Runtimeの独立ケース | 17件成功 | 規格から作成した既存スイート |
| 内部RuntimeのChibi由来ケース | 62件成功 | 出典を記録した既存スイート |
| Chibi原本のフォーム | 1,188フォームを試行 | import以外の全トップレベルフォーム |
| 原本で実行した判定 | 1,214件 | 成功1,213件、保留1件 |
| 未採用の任意指数表記 | 10フォーム | readで停止し、内側の20判定は未到達 |
| Guile 3.0.9との比較 | 17件と62件が成功 | 同じ独立ケースと期待出力。原本全体の比較ではない |
| 公開CLI（#66時点） | 17件と62件が失敗 | 旧処理系の別測定。内部の成功で置き換えない |
| 公開CLI（#67） | 17件と62件が成功 | 移行後のCLIを別プロセスで実測。CIでも回帰を検証 |

原本のsuite_passedはfalseのまま保存します。
no_unexpected_failuresは、既知の未採用と保留を除いた箇所に新しい失敗がなかったことだけを表します。
Goの監査テストはこの集計契約も検査するため成功しますが、その成功を原本スイートの全件成功とは呼びません。

## Chibi原本のアダプタ

[保存済み原本](https://github.com/0x6d61/guri/blob/master/testdata/r7rs/upstream/chibi/r7rs-tests.scm)は変更せず、SHA-256を確認してから実行します。
リビジョンはb2087f3a320a56f79ede5c546fd45fdde582c4ed、SHA-256は004d215a38f990bbf2bfaa8b4a395aa59ef98d155096f8e31cc361b7812f7d6aです。

Chibi本体やchibi/test.scmは追加していません。
Guri側で作成したテスト用ライブラリが、test、test-values、test-assert、test-error、test-begin、test-endだけを提供します。
テスト用の名前を標準ライブラリや公開CLIへ追加しません。

原本をdatumとして読み、トップレベルを同じ環境で順にコンパイルして実行します。
前方参照のセルを保持し、あるフォームのエラーで後続の独立したフォームまで未観測にしないようにしています。
先行する定義や副作用は共有するため、個々の判定が独立したプロセスであるという保証はありません。
通常プログラムの一括コンパイルと、この測定用の逐次実行も区別します。

比較は原則としてequal?相当です。
ルートの不正確数では、原本の丸めた例を扱うため、絶対誤差または相対誤差1e-6以内を許容します。
正確数をfloat64へ丸めて比較せず、隣接する大きな整数を同じ値として扱わない回帰テストを置いています。
この許容値はChibiのテスト実装を複製したものでも、Guriの=やeqv?の仕様を変更するものでもありません。

## 未採用と保留の分類

[R7RS §6.2.5](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.2.5)のs、f、d、lによる精度指定は任意です。
承認済みの数値プロファイルはbinary64であり、これらは採用していません。
原本の該当10フォームは、実際のread-errorを記録した上でnot-adoptedに分類します。
各フォーム内の2判定に到達しなかったことも別に記録します。
任意表記を通すためにReaderへ拡張を追加してはいません。

原本のr7rs featureの判定はdeferredです。
内部Runtimeの監査が進んでも、公開CLIの移行と準拠判定が済むまでは広告しません。
この保留をpassに書き換えたり、期待値を#fへ変更したりはしません。
分類する行と入力は原本の固定ハッシュに対応付け、別のエラーを無条件に既知の失敗へ分類しないよう検査します。

applyの4件のtest-errorは、[§1.3.2](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-3.html#TAG:__tex2page_sec_1.3.2)の検出義務と照合しています。
規格が「エラーである」とする条件と、「エラーを通知する」としてraiseを要求する条件は異なります。
この4件の引数数や不正Listは、検出したGo診断を記録します。
read-error?やfile-error?など通知が要求される箇所には、別途Schemeハンドラを使うテストがあります。
VMの全診断が一律にSchemeのraiseへ接続されていないことだけで、全件を必須不一致とは判定しません。

## 修正したマクロの動作

マクロが同じbeginへ導入する関数同士を、定義の前後にかかわらず同じセルへ解決するようにしました。
導入したprivate関数を利用者の同名の公開束縛と共有しないことも検証します。
define-valuesとRecordの生成手続きにも、同じ前方参照の処理を適用します。

```scheme
(import (scheme base))
(define-syntax make-functions
  (syntax-rules ()
    ((_ f)
     (begin
       (define (f x) (helper x))
       (define (helper x) (* x x))))))
(make-functions square-it)
(square-it 10) ; => 100
```

また、入れ子のsyntax-rulesでは、パターン中の識別子がliteral一覧に属するかを、名前と構文上の文脈で判定します。
入力とliteralを照合するときの束縛比較とは分けます。
同名で未束縛という理由だけで、外側から渡されたパターン変数をliteralへ変えてしまう不具合を修正しました。

```scheme
(import (scheme base))
(let-syntax
    ((m (syntax-rules ()
          ((_ x)
           (let-syntax
               ((n (syntax-rules (k)
                     ((n x) 'matched)
                     ((n y) 'unmatched))))
             (n z))))))
  (m k)) ; => matched
```

この2種類の期待結果はGuile 3.0.9でも確認しました。
派生した回帰テストはChibiのBSD 3-Clauseの出典とライセンスを保持します。

## 結合と負荷の検証

Promise、dynamic-wind、例外ハンドラ、parameterize、call/ccを組み合わせ、再入時の動的状態とメモ化を確認しています。
map、複数値、applyと例外ハンドラの結合、readの例外時のwind退出、evalと循環グラフのread/write/equal?も検証します。

importしたプログラムのapply、call/cc、call-with-values経由の末尾呼び出しを各10万回実行し、VMフレーム数に上限を設けています。
既存のPromiseの保持ヒープ検証、巨大List、有限グラフの往復、マクロの共有と循環の検証も引き続き実行します。
これらの負荷テストは選択した経路の証拠であり、すべてのプログラムに対する形式的な証明ではありません。

## 要求と証拠の索引

[sections.csv](https://github.com/0x6d61/guri/blob/master/docs/r7rs/audit/sections.csv)で、報告書の全96見出しを実装、既存と追加のテスト、Wikiへ対応付けます。
節そのものの下位要求がない場合は、同じ実装作業の証拠であることをevidence_scopeに明記します。
第1章の規約と第7章の形式文法や意味論は、機能件数へ加算しません。

この索引は、規格のすべての文を独立したテストへ分解したものではありません。
見出しに証拠があることから準拠率100%を計算しないようにします。
要求台帳の初期baselineも当初の調査記録として保持し、新しい結果は下位要求と監査報告で区別します。

## OSと公開CLIの残件

今回、実行した環境はLinux/amd64です。
Darwin/arm64とWindows/amd64のポートパッケージはクロスコンパイルを確認しましたが、そのOS上での実行は確認していません。
Linux以外の対話ファイルのchar-ready?/u8-ready?用プローブは既知の未対応です。
この残件は[#95](https://github.com/0x6d61/guri/issues/95)で追跡します。
全OSで準拠するという宣言は行いません。

#66時点の公開CLIは旧処理系でした。#67で独自Guri機能を除去してCLIを切り替え、実際の公開CLIで17 pass／62 passを確認しました。
現段階で配布物全体をR7RS-small準拠と呼ぶことはできません。
OS別の未対応、準拠の宣言範囲とリリース判定は、この結果をもとにユーザー確認を受けます。
