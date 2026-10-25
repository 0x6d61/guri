# プロセス情報、終了と時計

#65でscheme process-contextの5手続きとscheme timeの3手続きを内部Runtimeへ接続しました。
これでAppendix Aの全16ライブラリ、557公開束縛を内部Runtimeからimportできます。
公開CLIへの接続は#67、言語全体の準拠監査は#66で行います。
基準は[R7RS §6.14](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.14)です。

## 手続き一覧

| 手続き | 引数数と型 | 返り値と変更 |
|---|---|---|
| `(command-line)` | 0 | コマンド名から始まるStringのList。文字列は変更禁止、Listは呼び出しごとに新しく作る |
| `(get-environment-variable name)` | 1、String | 現在のOS環境変数の値を変更禁止のStringで返す。存在しなければ#f |
| `(get-environment-variables)` | 0 | 現在のOS環境変数を表す連想List。各要素は名前と値のStringのPair。List、Pair、Stringをすべて変更禁止にする |
| `(exit)`、`(exit obj)` | 0または1、任意 | 未退出のdynamic-windのafterを実行して終了する。値を返さない |
| `(emergency-exit)`、`(emergency-exit obj)` | 0または1、任意 | dynamic-windのafterを実行せず終了する。値を返さない |
| `(current-second)` | 0 | TAIの時刻を表す不正確な実数。UTCに既知の補正値を足した近似 |
| `(current-jiffy)` | 0 | Runtime初期化時から経過した単調時計のナノ秒数を正確な整数で返す |
| `(jiffies-per-second)` | 0 | 正確な整数1000000000 |

これらは引数や既存のコレクションを変更しません。
exitではafterの実行に伴う利用者の副作用が発生します。
環境変数を設定する独自手続きは追加しません。

## コマンドライン（D07-C）

2026-10-25に、起動側が指定するコマンド名と引数を採用する案をユーザーが承認しました。
内部Runtimeは作成時のos.Argsを既定値として保持し、CLIが選んだ呼び出し内容をSetCommandLineで設定できるようにします。
この受け口は内部フロントエンド用であり、公開の組み込みAPIを追加するものではありません。
公開CLIのスクリプト名やオプションとの対応は#67で接続します。

名前と引数はUTF-8として扱い、元のGoのスライスを後から変更しても影響しないようコピーします。
返すStringは変更禁止です。
外側のListは新しく作るため、set-car!やset-cdr!で変更しても、次のcommand-lineには影響しません。

```scheme
(import (scheme base) (scheme process-context))
(define args (command-line))
(and (pair? args) (string? (car args))) ; => #t
```

## 環境変数（D07-C）

環境変数はRuntimeの作成時に固定せず、呼び出した時点のOSの値を読みます。
名前と値はUTF-8として解釈し、復号できないバイト列はエラーにします。
存在しない変数の#fと、存在するが空の値であるStringは区別します。

```scheme
(import (scheme base) (scheme process-context))
(define path (get-environment-variable "PATH"))
(or (not path) (string? path)) ; => #t
```

一覧の列挙順序は保証しません。
値に含まれる等号は保持し、Windowsのドライブ別環境変数のような先頭が等号の名前も保持します。
環境変数の取得結果はコピーして変更禁止にし、その内容を変更してOSの環境を操作することはできません。

```scheme
(import (scheme base) (scheme process-context))
(define vars (get-environment-variables))
(if (null? vars)
    #t
    (guard (e (else (error-object? e)))
      (set-car! vars 'changed)
      #f)) ; => #t
```

## 終了規則（D06）

ユーザー承認に従い、exitとemergency-exitは次の同じ変換規則を使います。
省略と真偽値の成功／失敗の区別はR7RSに従い、具体的な整数への変換はGuriの規則です。

| obj | 終了コード |
|---|---:|
| 省略または#t | 0 |
| #f | 1 |
| 正確な整数 | 256で割った0以上255以下の余り |
| その他の値 | 1 |

たとえばexitに7を渡すと7、256を渡すと0、-1を渡すと255です。
任意精度整数でも、機械整数へ丸めてから変換することはありません。
不正確数の0.0、String、空Listなどはその他の値として1にします。

exitは未退出のdynamic-windのafterを内側から順に実行します。
afterで複数値を返しても終了コードには使いません。
afterの中から別のexitやemergency-exitを呼んだ場合は、その新しい終了要求を適用します。

```scheme
(import (scheme base) (scheme write) (scheme process-context))
(dynamic-wind
  (lambda () (display "開始\n"))
  (lambda () (exit 7))
  (lambda () (display "後処理\n")))
```

この例は「開始」「後処理」を表示し、終了コード7で終了します。
呼び出し後のSchemeコードへ復帰せず、guardで捕捉するScheme例外にも変換しません。

```scheme
(import (scheme base) (scheme write) (scheme process-context))
(dynamic-wind
  (lambda () (display "開始\n"))
  (lambda () (emergency-exit 7))
  (lambda () (display "後処理\n")))
```

こちらは「開始」だけを表示して終了します。
緊急終了では、利用者がafterに書いたflushやcloseも実行しません。
通常終了でもすべてのポートを無条件にcloseする機能は、この手続き自体には含めません。
ポートの後処理は[ポートとファイルの規則](https://github.com/0x6d61/guri/wiki/Datum-File-IO)に従います。

内部実装はExitRequestをCLIへ渡す形にします。
目的は終了コード、後処理とREPLの停止をプロセスを落とさずテストすることであり、組み込み用途のAPI整備は対象外です。
REPLも終了要求を通常のエラーとして表示せず、次の入力を評価せずに返します。
#67で公開CLIが終了要求を受け取り、Guriのプロセスを終了させます。

## 時計（D07-T）

current-secondはUnixのUTC秒にTAIとUTCの差を加えた近似値です。
基準は1970年1月1日0時TAIで、返す値は不正確数です。
R7RSは高精度を要求せず、UTCに適切な定数を加える近似を認めています。

現在の実装は2017年1月1日以降のTAI−UTCである37秒を使います。
[USNOのTAI−UTC表](https://maia.usno.navy.mil/ser7/tai-utc.dat)を2026-10-25に確認しました。
ネットワークへの実行時アクセスは行いません。
将来のうるう秒で差が変わった場合は、この定数を更新する必要があります。
歴史的な日時ごとの補正や、うるう秒の瞬間を独立した秒として表す機能は提供しません。
OSの時計補正により値が戻る可能性があるため、経過時間の計測にはcurrent-jiffyを使います。

```scheme
(import (scheme base) (scheme time))
(and (real? (current-second))
     (inexact? (current-second))) ; => #t
```

current-jiffyはGoの単調時計を使い、Runtimeを初期化した時点を起点にします。
同じRuntime内では起点を変えず、RunやREPLを再度呼び出してもリセットしません。
別のRuntimeや別のプロセスのjiffyと直接比較するための絶対時刻ではありません。
1秒を10億jiffyとしますが、OSが1ナノ秒単位の測定精度を保証するという意味ではありません。

```scheme
(import (scheme base) (scheme time))
(define start (current-jiffy))
(define elapsed (- (current-jiffy) start))
(and (>= elapsed 0)
     (= (jiffies-per-second) 1000000000)) ; => #t
```

## エラーと提供範囲

環境変数の名前の型が違う場合、UTF-8に復号できない値や不正なOS環境エントリがある場合は、通常のerror-objectを送出します。
引数数の不一致は既存VMと同じGo診断であり、自動raiseへの接続は引き続き監査対象です。
終了要求はerror-objectではありません。

file-exists?とdelete-fileは#61の実装を再利用します。
使用方法とfile-error?は[ライブラリの仕様](https://github.com/0x6d61/guri/wiki/Libraries)を参照してください。

featuresは引き続きguri、exact-closed、exact-complex、ieee-float、full-unicode、ratiosを提供します。
これらは既存の数値プロファイルとUnicodeの対応を表し、r7rsはまだ広告しません。
全16ライブラリの公開識別子を台帳と照合することと、言語全体の準拠完了は区別します。
公開CLIの測定は17 fail／62 failを維持し、#66/#67で監査と移行後の測定を行います。
