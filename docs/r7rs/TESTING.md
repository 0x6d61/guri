# R7RSテストの実行

#44で追加する基盤は、Guriの評価器と独立したGoの実行器です。
各ケースのSchemeソースを別プロセスで実行し、期待する標準出力と終了コードに照合します。
規格を参照して独立に作成した17ケースと、Chibi由来の62ケースを別のマニフェストで管理します。
規格全体を網羅するスイートではありません。

## unitテストと規格ケースの分離

```sh
go test ./...
go vet ./...
```

unitテストは外部のScheme処理系やネットワークを使いません。
実行器のテストではGoのテスト実行ファイルを補助プロセスとして起動し、出力比較、終了、タイムアウト、出力上限、未対応の集計と設定エラーを検証します。
Guriの言語機能が規格に準拠しているかは、この基盤のunitテストの成功とは別の結果です。

## 規格ケースの測定

リポジトリのルートで実行します。
既存の実行ファイルを上書きしないよう、出力先を一時ディレクトリにします。

```sh
r7rs_work=$(mktemp -d)
go build -o "$r7rs_work/guri" ./cmd/guri
go build -o "$r7rs_work/r7rscheck" ./cmd/r7rscheck
"$r7rs_work/r7rscheck" -program "$r7rs_work/guri" -report-only > "$r7rs_work/report.json"
```

`-report-only`では、言語機能による不一致やタイムアウトがあっても、結果を生成できた場合は終了コード0にします。
JSONの失敗件数と`suite_passed`は書き換えません。
起動失敗、設定不備、キャンセル、出力上限超過など、実行基盤に関わるエラーは終了コード2です。

現行Guriはimportと標準ライブラリを持たないため、初期の17ケースは失敗します。
それをskipやxfailに変えずに測定します。
ソースは標準のプログラムとして実行するため、importを独自構文へ書き換えて成功率を上げることはしません。
Readerや評価器など個々の機能のTDDは、それぞれのunitテストでも行います。

失敗を終了コードへ反映する検査では、`-report-only`を付けません。

```sh
"$r7rs_work/r7rscheck" -program "$r7rs_work/guri" > "$r7rs_work/strict-report.json"
```

終了コードは、全選択ケースがpassなら0、それ以外は1、実行基盤のエラーなら2です。
これは選択したケースの成否であり、規格全体の準拠宣言ではありません。
ケースを選択して最小の再現を得ることもできます。

```sh
"$r7rs_work/r7rscheck" -program "$r7rs_work/guri" -case apply-final-list -timeout 2s
```

実行器はソースのパスを実行ファイルの最後の引数に追加します。
処理系に前置引数が必要なら`-arg=値`を繰り返して指定します。
シェル展開は行いません。
各プロセスの実行時間は既定で5秒、標準出力と標準エラーはそれぞれ1 MiBまでです。
タイムアウトはそのプロセスを停止します。
子孫プロセス全体を管理する機能はないため、実行対象には処理系本体を指定します。

## マニフェストと要求ID

`testdata/r7rs/suite.json`を入口にし、ケースごとのソースを同じディレクトリに置きます。
ソースのパスはマニフェストからの相対パスです。
各ケースにID、要求ID、分類、出典、比較方法、期待出力と終了コードを記載します。
要求IDの節部分は`docs/r7rs/requirements.csv`へ照合します。
未知の節、重複したケースID、不正な比較方法や欠落したファイルは設定エラーです。

| フィールド | 用途 |
|---|---|
| `id` | `-case`で選択できる一意のID |
| `requirement_id` | `R7RS-6.10/apply-final-list`など、節と下位条件のID |
| `class` | required、optional、implementation-defined、unspecified、error-required、error-optional |
| `origin` | 規格や外部ソースの固定版と位置 |
| `file` | 独立して実行できるSchemeソース |
| `match` | exact、one-of、observe |
| `stdout`、`alternatives` | 正確な出力、または許容される複数の出力。空白と改行も比較する |
| `exit_code` | 正常に観測できたプロセスの期待終了コード |
| `disposition` | 省略またはrunで実行。skip、unsupported、restricted、xfailは理由と追跡Issueが必要 |
| `reason`、`issue` | 未完了の理由と追跡先 |

`class`だけで結果を成功にしたり、任意ライブラリの提供を確定したりしません。
optionalの失敗も選択したスイートの失敗として報告します。
規格が検出を要求しないエラーはerror-optionalとして分けます。
例外の検出が必須のケースでは、単にプロセスが落ちることを期待値にせず、Schemeのハンドラで例外と述語を確認して所定の出力を返します。

未規定の結果は、one-ofで許容される結果を列挙するか、observeで観測だけを記録します。
operand-orderでは12と21のどちらも認め、評価順を一方に固定しません。
結果の具体値ではなく性質を検査する場合は、その性質を要求IDとソースに記載し、性質の要件をrequiredとして自己検査の出力をexactで比較します。
observeは必須結果の代用には使えず、成功件数にも入りません。
実装依存の期待値には、承認されたプロファイルの根拠も記録します。

## JSONの結果

`schema_version`は1です。
`results`の各項目に要求ID、分類、出典、完全な起動引数、再現用ソース、期待値、実際の出力と終了コード、実行時間を含めます。
`counts`は全体、`chapters`は章、`sections`は節ごとの結果を集計します。
`uncovered_sections`はケースが一つもない節の一覧で、そこを検証済みにはしません。
同じ節の一部のケースが成功したことも、その節全体の検証完了を意味しません。
件数を規格の準拠率に換算しません。

| status | 意味 |
|---|---|
| pass | 期待する出力と終了コードを満たした |
| fail | 通常終了または非ゼロ終了を観測したが、期待値と不一致 |
| timeout | 時間制限を超えた。xfailには変換しない |
| error | 起動、設定、キャンセル、出力制限などの実行上のエラー |
| skip、unsupported、restricted | 実行していないケース。理由とIssueを記録する |
| xfail | 実行した結果が既知の不一致に一致した。成功には含めない |
| xpass | 既知の不一致としていたケースが通った。指定を見直すまで成功には含めない |
| observed | 未規定値などの観測だけを行った。検証成功には含めない |

一件でもpass以外があれば`suite_passed`はfalseです。
全ケースがpassでも、記載された選択ケースが成功したことだけを表します。
`requirements.csv`や`exports.csv`の状態を自動で検証済みに変えません。
証拠として採用する際は、実装PRで要求ID、テスト名、結果と範囲を確認します。

## CI

GoのCIは従来のunitテストとvetに加え、`-report-only`で測定します。
測定ステップの名前にも、準拠判定のゲートではないことを記載します。
実行結果のJSONを`r7rs-measurements`というArtifactへ保存します。
独自作成ケースは`r7rs-report.json`、Chibi由来ケースは`r7rs-chibi-report.json`です。
CIが緑でも、JSONのfailや未測定の節は未対応として残ります。
将来、必須ケースが揃った段階でstrict実行をゲートに加えます。

## 外部スイートの比較

| 候補 | 確認した固定版 | 特徴と判断 |
|---|---|---|
| [Chibi Scheme](https://github.com/ashinn/chibi-scheme/tree/b2087f3a320a56f79ede5c546fd45fdde582c4ed) | `b2087f3a320a56f79ede5c546fd45fdde582c4ed` | tests/r7rs-tests.scmはUnicode全体、数値塔と任意ライブラリを前提にする。採用済み。原本を保存し、規格と照合したbaseの62ケースを独立実行する |
| [Gauche](https://github.com/shirok/Gauche/blob/0d709aafd44354593817ecc085adb82697b102d5/tests/r7rs-tests.scm) | `0d709aafd44354593817ecc085adb82697b102d5` | Chibiのテストをgauche.testへのアダプタで使い、追加の回帰テストも持つ。Gauche固有の環境を今回の基盤へ追加する予定はない |

ChibiのCOPYINGはBSD 3-Clauseです。
ソースを取り込む場合は元の著作権表示、ライセンス条件、免責文とファイル内の寄稿者表示を保持します。
GaucheにもBSDのCOPYINGがありますが、取り込むファイルが決まった時点で個別の条件を確認します。
Guri本体のMITライセンスと外部由来の条件を混同しません。
ユーザーの取り込み指示により、C44-01を承認済みとして[Issue #70](https://github.com/0x6d61/guri/issues/70)で実装します。
[原本とCOPYING](../../testdata/r7rs/upstream/chibi/README.md)、[派生ケース](../../testdata/r7rs/chibi/README.md)、[外部ライセンス表示](../../THIRD_PARTY_NOTICES.md)を管理します。

### Chibiの取り込み範囲

取り込む原本は`tests/r7rs-tests.scm`とルートの`COPYING`です。
リビジョンは上表の`b2087f3a320a56f79ede5c546fd45fdde582c4ed`に固定し、原本を改変せず`testdata/r7rs/upstream/chibi/`へ保存します。
追加依存のある`lib/chibi/test.scm`とそのライブラリ群は、この段階では取り込みません。

| ファイル | SHA-256 |
|---|---|
| tests/r7rs-tests.scm | `004d215a38f990bbf2bfaa8b4a395aa59ef98d155096f8e31cc361b7812f7d6a` |
| COPYING | `86805fe7af1bd20d5f5050d86c61d6fe038452cb08c6106a08761e010d37da43` |

出典とライセンスの範囲を同じディレクトリの説明に記録し、元の寄稿者表示も保持します。
独立ケースへ移すときは原本の行、規格の節と変更内容を記録し、派生したファイルもBSDの表示対象として扱います。
この変更はテストの取り込みであり、Guriの実行コードは変更しません。
原本全体は`(chibi test)`への依存があるため、この取り込みだけで全件を実行できるとは記載しません。

### 派生ケースの測定

既存の17ケースと同じ実行器へ、Chibi用のマニフェストを指定します。

```sh
"$r7rs_work/r7rscheck" -program "$r7rs_work/guri" -suite testdata/r7rs/chibi/suite.json -report-only > "$r7rs_work/chibi-report.json"
```

初回のGuri測定では62件がfailになります。
元のアサーションは変更せず、baseのimportとローカルなtestマクロを追加しています。
マクロは選択した正確な値をequal?で比較し、期待値と式を一度ずつ評価します。
近似Float比較や任意ライブラリを必要とするケースは、この初回の独立実行対象へ含めません。
これは数値やUnicodeの実装プロファイルの決定を意味しません。

`provenance.json`から元の行範囲、規格の節と追加した実行用コードを追跡できます。
原本全体の保存と、この62ケースの実行範囲を区別します。
今後ケースを追加するときも、未規定の結果、任意機能、数値の近似比較を個別に確認します。

### 検証の範囲

CIとunitテストにChibi本体を追加しません。
取り込んだ62ケースを、既存のGo実行器からGuriへ渡して測定します。
原本の固定、ライセンスと元のアサーションの保持、要求IDの対応はunitテストで確認します。

現行Guriはimportなどが未対応のため、62ケースすべてが失敗します。
この測定だけで各ケース内部の動作を検証できたとは扱いません。
派生した実行用コードをChibiなどの外部処理系で実行する検証は、今回の変更には含めません。

## 標準Readerの検査

#46で、全79件のソースを新しいinternal/readerで読み切るunitテストを追加しました。
この検査は、言語の実行結果を測定する既存のJSONとは別です。
Readerのテストが成功しても、現行CLIの79件をpassへ変更しません。

Unicode case foldingにGo公式のx/text/cases v0.43.0を使用します。
初回は`go mod download`で依存を取得し、その後は`GOPROXY=off GOSUMDB=off go test ./...`でネットワークなしの検証ができます。
CIでも依存の取得とunitテストを別ステップにします。

## 内部Runtimeでの原本監査（#66）

Chibi本体を追加せず、保存した原本とGuri側のテストアダプタを使います。
公開CLIの測定とは別に実行し、プロセスの測定結果を上書きしません。

```sh
GURI_AUDIT_REPORT=/tmp/internal-r7rs-audit.json \
  GOPROXY=off GOSUMDB=off go test ./internal/core -run '^TestChibiSourceAudit$' -count=1 -v
```

原本の全トップレベルフォームを順に試行し、各判定、実際のエラー、未採用の理由と未到達件数をJSONへ記録します。
任意の精度指定の指数表記は10フォームでread-errorとなり、各2判定は未到達です。
r7rs featureは広告保留のため1判定が未成功で、原本のsuite_passedはfalseです。
監査のunitテストは、この既知の分類と新しい不一致の不在を確認する契約です。
成功件数と原本の合否を混同しないでください。

### 別処理系での既存ケースの確認

開発時にはGuile 3.0.9で既存の独立17件とChibi由来62件を実行し、両方の期待出力に一致しました。
Guileは一時ディレクトリへ展開して使用し、GuriのソースやCI依存には追加していません。
インストール済みのGuileで再確認する場合は、次のように実行できます。

```sh
go run ./cmd/r7rscheck -program guile -arg=--r7rs -arg=--no-auto-compile
go run ./cmd/r7rscheck -program guile -arg=--r7rs -arg=--no-auto-compile \
  -suite testdata/r7rs/chibi/suite.json
```

これは原本全体の差分比較ではありません。
参照処理系の成功も、規格全体に対するGuriの準拠宣言には使いません。
