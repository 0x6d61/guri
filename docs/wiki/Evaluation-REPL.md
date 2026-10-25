# 環境、eval、loadとREPL

#64で環境を指定する評価と対話環境を内部Runtimeへ接続しました。
公開CLIと公開Go APIの切り替えは#67で行います。
基準は[R7RS §6.12](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.12)、[loadの定義](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.14)、[REPLの定義](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-7.html#TAG:__tex2page_sec_5.7)です。

## 構文と手続き

| 手続き | 引数数と型 | 返り値と変更 |
|---|---|---|
| `(environment import-set ...)` | 0以上。各引数はimport setを表すdatum | 空環境へ指定したライブラリをimportした、変更禁止の環境指定子。ライブラリの初期化を行う |
| `(eval expr-or-def environment-specifier)` | 2。式または定義のdatumと環境指定子 | 指定環境で評価した0個以上の値。可変環境なら定義と代入で束縛を変更できる |
| `(interaction-environment)` | 0 | 同じRuntimeで共有する可変の対話環境指定子 |
| `(scheme-report-environment version)` | 1。正確な整数5 | scheme r5rsの束縛だけを持つ、変更禁止の環境指定子 |
| `(null-environment version)` | 1。正確な整数5 | scheme r5rsの構文束縛だけを持つ、変更禁止の環境指定子 |
| `(load filename)` | 1。String | 対話環境でファイルを実行する。返り値は未規定値 |
| `(load filename environment-specifier)` | 2。Stringと環境指定子 | 指定環境でファイルを実行する。返り値は未規定値 |

scheme evalはenvironmentとeval、scheme replはinteraction-environment、scheme loadはloadを公開します。
scheme-report-environmentとnull-environmentはscheme r5rsから公開します。
#65のscheme process-contextとscheme timeを含め、内部Runtimeでは全16標準ライブラリを提供します。

## datumとソースの違い

evalにはReaderが返すdatumや、listなどで構築したdatumを渡します。
文字列をソースコードとして読み直すことはありません。
式に現れる名前は指定環境で解決し、呼び出し元のローカル束縛を暗黙には持ち込みません。

```scheme
(import (scheme base) (scheme eval))
(eval '(* 7 6) (environment '(scheme base))) ; => 42
```

```scheme
(import (scheme base) (scheme eval))
(let ((x 100))
  (eval '(let ((x 40)) (+ x 2))
        (environment '(scheme base)))) ; => 42
```

```scheme
(import (scheme base) (scheme eval))
(eval "(+ 1 2)" (environment)) ; => "(+ 1 2)"
```

構築したコードを表示してから再読込する実装ではなく、datumの共有と循環を保った構文グラフをコンパイラへ渡します。
quoteの内部では同じオブジェクトを保持し、実行コード側の循環はエラーにします。
定数のString、Vector、Bytevectorと引用データには、既存のリテラル変更禁止規則が適用されます。

## 環境の変更と共有

environmentはonly、except、rename、prefixを含むimport setを受け取ります。
指定しない束縛は利用できません。
初期化中のライブラリを動的に再要求した場合は、再帰し続けずエラーにします。
元のライブラリと同じ束縛セルを参照するため、ライブラリ内の手続きが変更した状態は環境からも見えます。
変更禁止なのは環境とその束縛であり、値として保持する可変オブジェクトの内容まで複製したり凍結したりはしません。

```scheme
(import (scheme base) (scheme eval))
(eval '(sum 40 2)
      (environment '(rename (only (scheme base) +) (+ sum)))) ; => 42
```

```scheme
(import (scheme base) (scheme eval))
(guard (e (else (error-object? e)))
  (eval '(define answer 42) (environment '(scheme base)))) ; => #t
```

evalの対象がlambdaやletの本体なら、変更禁止の環境でもローカルな内部定義とset!を利用できます。
トップレベルへのdefine、define-values、define-syntax、define-record-typeによる追加や置換は拒否します。
scheme-report-environmentとnull-environmentにも、この実装では変更禁止を適用します。
version 5以外は提供しません。

```scheme
(import (scheme base) (scheme r5rs))
(define f (eval '(lambda (f x) (f x x)) (null-environment 5)))
(f + 21) ; => 42
```

## 対話環境（D07-E）

ユーザー承認に従い、対話環境には提供済み標準ライブラリの束縛を初期配置します。
同じRuntime内では対話環境を共有し、別のRuntimeへ定義を持ち越しません。
通常のプログラムは、従来どおりimportで始まる空の環境から実行します。

初期の標準変数には対話用の束縛セルを用意します。
対話環境の再定義やset!は、通常のプログラムと標準ライブラリの束縛を変更しません。
対話環境で標準束縛を別名としてimportした場合は、同じ対話用セルを共有します。
手続きやポートなどの値自体は浅く共有し、資源や手続きの定義環境を複製しません。

```scheme
(import (scheme base) (scheme eval) (scheme repl))
(define e (interaction-environment))
(eval '(define answer 40) e)
(eval '(set! answer (+ answer 2)) e)
(eval 'answer e) ; => 42
```

```scheme
(import (scheme base) (scheme eval) (scheme repl))
(eval '(set! + (lambda args 99)) (interaction-environment))
(list (eval '(+ 1 2) (interaction-environment))
      (+ 20 22)) ; => (99 42)
```

REPLでは定義や式の間にimportできます。
後からのimportによる名前の置換と、importした名前への対話側の再定義を認めます。
標準以外のライブラリを明示importした場合、その束縛セルはライブラリと共有します。
また、関数本体が後から定義する名前を参照する形も認めます。
未初期化の名前を実際に参照した場合はエラーになります。

## loadのファイル規則（D07-E）

loadはファイルのソースを読み、式と定義を順番にコンパイルして実行します。
ライブラリの公開名を取り込むimportとは用途が異なります。
前の式で定義したマクロを、同じファイルの後の式から使えます。
省略時は対話環境を使用するため、load後もそこで定義した関数を利用できます。
明示した環境が変更禁止なら、新しいトップレベル定義はエラーになります。

パスはOSのパスとして扱い、相対パスは実行時の作業ディレクトリを基準にします。
GURI_LOAD_PATH、インストール先、呼び出し元のスクリプトの隣は検索しません。
UTF-8のソースだけを読み、コンパイル済みバイナリは対象にしません。
結果の自動表示は行わず、current-input-portとcurrent-output-portを差し替えません。

loadはファイルの内容をメモリへ読み込んで閉じ、読み取った式を順に実行します。
後続のdatumでReaderエラーが見つかった場合、それより前に読み取った式は実行した後、そのエラーを送出します。
ただしUTF-8の検証は入力全体に対して先に行います。
途中で失敗しても、既に実行した定義や副作用を巻き戻しません。
空のファイルでも返り値は未規定値です。

たとえばtools.scmを次の内容で保存します。

```text
(define (answer) 42)
```

REPLで次のように使えます。

```text
(load "tools.scm")
(answer)
; => 42
```

## 複数値、動的環境と継続

evalは呼び出し時の動的パラメータと例外ハンドラを引き継ぎます。
末尾位置のevalは、コンパイルしたコードの復帰を待つ余分な継続を追加しません。
10,000回のevalを経由する末尾呼び出しでVMフレーム数を検証します。

```scheme
(import (scheme base) (scheme eval))
(call-with-values
  (lambda () (eval '(values 20 22) (environment '(scheme base))))
  +) ; => 42
```

evalやloadの中でcall/ccが捕捉した継続は、元の呼び出しが終了してからも同じRuntimeで複数回再開できます。
loadは読み取ったソースと次の式の位置を継続に保持し、再開時にはその位置以降を再実行します。
既に閉じたファイルを再度読むことはありません。
明示的に再開した式の副作用は再び発生します。

## 内部REPLの入出力

内部Runtime.REPLは1つのdatumずつ読み、同じ対話環境で評価します。
0個の返り値と未規定値は表示せず、その他の値はwriteの表記で1行ずつ表示します。
表示用プロンプトはフロントエンドの担当とし、この内部ループには含めません。

読み取り、コンパイル、評価の通常エラーは診断出力へ表示し、次の入力へ進みます。
不正な入力でReaderが進まなかった場合は、その文字を消費して再試行します。
EOF、キャンセル、入出力先そのものの障害ではループを終了します。
任意のGo Readerが入力待ちで停止している間のcontext割り込みは保証せず、入力アダプタの契約に従います。

REPLのcurrent-input-portはReaderと位置を共有し、readやread-charで同じ入力を消費します。
current-output-portとcurrent-error-portには渡された出力先を動的に設定します。
Goのファイル入力には既存のファイル用アダプタを使います。
ready?の対応範囲は[ポートの仕様](https://github.com/0x6d61/guri/wiki/Ports)に従います。
Goから渡されたストリームは借用し、REPLの終了時に元のストリームを閉じません。

## エラーと未接続の範囲

不正な環境指定子、import set、未対応version、変更禁止の環境への定義や代入、eval対象の構文エラーは通常のerror-objectを送出します。
loadのファイル読取失敗はfile-error?、ソースのReader失敗はread-error?で識別できます。
エラーには利用できるファイル名と位置を残し、datumのevalでは呼び出し位置を使います。

既存VMの引数数不一致や未初期化セルの実行時参照など、Go診断からSchemeの自動raiseへ未接続の経路は継続して監査対象とします。
REPLはこれらの診断も表示して次の入力へ進みます。
この内部実装の成功を、公開CLIの準拠測定の成功や公開Go APIの移行完了とは扱いません。
exitとemergency-exitは#65で接続済みです。
CLIの入力モード、表示とプロセス終了は#67で接続します。
