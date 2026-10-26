# R7RS適合性テストの実行と結果

`r7rscheck`は、規格を参照して作ったケースを独立したプロセスで実行するGoの検証ツールです。
GuriのReaderや評価器の内部構造には依存しません。
現行のGuriはR7RS-smallに準拠しておらず、このツールを追加しても標準機能が使えるようになるわけではありません。

## 実行方法

リポジトリのルートで実行します。
Goのunitテストとvetは、従来どおりに実行します。

```sh
go test ./...
go vet ./...
```

規格ケースの測定では、既存のバイナリを上書きせず、一時ディレクトリへビルドします。

```sh
r7rs_work=$(mktemp -d)
go build -o "$r7rs_work/guri" ./cmd/guri
go build -o "$r7rs_work/r7rscheck" ./cmd/r7rscheck
"$r7rs_work/r7rscheck" -program "$r7rs_work/guri" -report-only > "$r7rs_work/report.json"
```

初期コーパスの17ケースは、規格に従うimportと標準手続きを使います。
現行Guriでは未対応なので、17件のfailを報告し、`suite_passed`はfalseです。
`-report-only`の終了コード0は測定処理の完了を示し、規格テストへの合格を意味しません。
JSONの失敗件数は保持します。

`-report-only`を外すと、全選択ケースがpassなら終了コード0、それ以外なら1です。
設定や起動などの実行エラーは、どちらのモードでも2です。
一つのケースだけを再現する場合は`-case apply-final-list`のように指定します。

## 結果の分類

JSONに要求ID、規格の節、出典、起動引数、ソース、期待値、実際の出力と終了コード、実行時間を残します。
章と節ごとに集計し、ケースのない節も一覧にします。
実行時間の上限はケースごとに既定で5秒、出力は標準出力と標準エラーをそれぞれ1 MiBに制限します。

pass、fail、timeout、error、skip、unsupported、restricted、xfail、xpass、observedを区別します。
pass以外を成功件数に含めません。
必須、任意、実装依存、未規定、検出が必須のエラーと必須でないエラーも分けて記録します。
未規定の評価順は複数の許容結果を認め、単一の評価順を規格の正解にしません。

## CIと対応範囲

CIはGoのテストとvetに加え、規格ケースを`-report-only`で測定します。
JSONは`r7rs-measurements`というArtifactから取得できます。
CIが成功しても、測定結果に残るfailを準拠済みと扱いません。
全選択ケースが通っても、規格全体の網羅や準拠を意味しません。

外部スイートの候補比較、ライセンス、マニフェストの形式と詳細な実行方法は[原稿の手順書](https://github.com/0x6d61/guri/blob/master/docs/r7rs/TESTING.md)に記載します。
現在の言語機能は[言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification)、移行方針は[準拠計画](https://github.com/0x6d61/guri/wiki/R7RS-Conformance)で確認できます。

## Chibiのテスト取り込み

[Issue #70](https://github.com/0x6d61/guri/issues/70)で、固定版Chibiのテスト原本とCOPYINGを保存し、62件を独立実行できる形にしました。
既存の17件は独自作成、追加した62件はChibi由来です。
原本全体を保存したことと、原本の全ケースを実行できることは区別します。

各派生ファイルでは元のアサーションを残し、baseのimportと比較用のtestマクロを追加します。
出典の固定リビジョンと行番号、変更内容、BSDのライセンス表示を保持しています。
[外部ライセンス表示](https://github.com/0x6d61/guri/blob/master/THIRD_PARTY_NOTICES.md)と[出典の対応表](https://github.com/0x6d61/guri/blob/master/testdata/r7rs/chibi/provenance.json)から確認できます。

前述のビルド後に、Chibi由来のケースを測定できます。

```sh
"$r7rs_work/r7rscheck" -program "$r7rs_work/guri" -suite testdata/r7rs/chibi/suite.json -report-only > "$r7rs_work/chibi-report.json"
```

現行Guriの測定結果は62件のfailです。
CIのArtifactには、独自作成ケースとChibi由来ケースのJSONを別々に保存します。
Chibi本体は取り込まず、CIにも追加しません。
元のアサーションと出典の対応はunitテストで確認し、Guriでの動作は測定結果として残します。
現時点では各ケース内部の動作や外部処理系での成功まで確認したとは扱いません。

## 内部Runtimeによる結合検証

#61で、独自17件とChibi由来62件を内部Runtimeでも実行し、同じ期待出力に対して79件が成功しました。
TestImportedProgramsFromConformanceSuitesは各ケースに独立したRuntimeと出力ポートを与え、exactとone-ofを照合します。
現在のケースはいずれも終了コード0であり、OSプロセスの終了状態を測るテストとは区別します。
#67で公開CLIを移行して再測定し、JSONは17 pass／62 passとなりました。#66までの旧処理系の17 fail／62 failは過去の測定として残します。
内部のunitテスト結果で公開CLIのレポートを置き換えたり、準拠完了を宣言したりしません。

- [R7RS-smallの結合監査](https://github.com/0x6d61/guri/wiki/R7RS-Audit): 原本の測定結果、修正した衛生性、96見出しの証拠索引と残る制限。

## OS別のCI

#95ではLinux、macOS、Windowsの標準ランナーで全Goテストとvet、公開CLIの17件と62件を実行します。
OSごとのArtifactに実行環境とGoテストのJSON、2種類のCLI測定、原本監査を保存します。
実行方法はリポジトリの`docs/r7rs/TESTING.md`、確認した環境は[結合監査](https://github.com/0x6d61/guri/wiki/R7RS-Audit)を参照してください。
