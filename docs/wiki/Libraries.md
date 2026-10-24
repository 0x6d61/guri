# ライブラリとプログラムの環境

内部Runtimeにdefine-libraryとimportを接続しました。
プログラムは空の環境から始め、importした束縛と自分で定義した名前を使用します。
公開CLIと公開Go APIの切替は#67で行います。
REPLとevalの環境は#64で内部Runtimeへ接続済みです。
テスト用のcore.NewCoreは従来どおり内部機能をまとめた環境であり、標準プログラムの初期環境とは区別します。
基準は[R7RS-small 第5章](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-7.html)です。

## 対応範囲（D03）

2026-10-24に、全16標準ライブラリを目標にして完成したものから段階提供する方針を承認しました。
現在は以下の14ライブラリを内部Runtimeのimportで利用できます。

| ライブラリ | 公開束縛数 | 内容 |
|---|---:|---|
| `(scheme base)` | 238 | コア構文、制御、基本の数値とコレクション、ポート |
| `(scheme case-lambda)` | 1 | case-lambda |
| `(scheme char)` | 22 | Unicode分類と文字・文字列のcase folding |
| `(scheme complex)` | 6 | 複素数 |
| `(scheme cxr)` | 24 | 深さ3と4のPairの取得 |
| `(scheme eval)` | 2 | 環境指定とdatumの評価 |
| `(scheme file)` | 10 | ファイル入出力、存在確認と削除 |
| `(scheme inexact)` | 12 | 不正確数の数学関数と分類 |
| `(scheme lazy)` | 5 | Promiseと遅延評価 |
| `(scheme load)` | 1 | ソースファイルの実行 |
| `(scheme read)` | 1 | datumのread |
| `(scheme repl)` | 1 | 対話環境 |
| `(scheme r5rs)` | 222 | R5RSの互換ライブラリ |
| `(scheme write)` | 4 | write、write-shared、write-simple、display |

公開識別子はdocs/r7rs/exports.csvと内部の束縛表をテストで照合します。
scheme lazyは#63で接続済みです。
scheme eval／load／repl／r5rsは#64で接続済みです。
scheme process-context／timeは#65で接続します。
未提供の2ライブラリを空のライブラリとしてimportできるようにはせず、library requirementでも利用可能と広告しません。
14ライブラリの接続を、言語全体の準拠完了として扱いません。

## define-libraryと宣言

```scheme
(define-library (myapp counter)
  (export count next (rename next step))
  (import (scheme base))
  (begin
    (define count 0)
    (define (next)
      (set! count (+ count 1))
      count)))
```

ライブラリ名は識別子と非負の正確な整数からなる空でないListです。
整数の1とSymbolの`|1|`は異なる名前要素です。
SchemeやSRFIの予約名の規則は報告書に従い、組み込み標準ライブラリ名の再登録はエラーにします。
上記はライブラリ定義の例であり、プログラムのimportより前へ直接並べる形式は採用しません。
内部RuntimeのRegister、または検索した.sldファイルから定義を登録します。
登録だけではライブラリ本体を実行しません。

| 宣言 | 引数・内容 | 動作 |
|---|---|---|
| `(define-library name declaration...)` | ライブラリ名と0個以上の宣言 | 新しいライブラリ定義を登録する |
| `(export spec...)` | 識別子、または`(rename internal external)` | 定義済み／import済みの束縛を外部名で公開する |
| `(import import-set...)` | 0個以上のimport set | 他ライブラリの公開束縛を取り込む |
| `(begin form...)` | 0個以上の定義や式 | ライブラリ本体として順にコンパイル・実行する |
| `(include filename...)` | 1個以上のStringリテラル | ファイル中の定義や式を本体へ追加する |
| `(include-ci filename...)` | 1個以上のStringリテラル | case foldingを有効にして本体へ追加する |
| `(include-library-declarations filename...)` | 1個以上のStringリテラル | ファイル中のライブラリ宣言を追加する |
| `(cond-expand clause...)` | feature requirementと宣言の節 | 最初に満たす節の宣言を展開する。elseは最後だけに置く |

宣言のcond-expandを展開し、全importを処理してから、begin／include／include-ciの本体を記述順にコンパイルします。
exportは束縛そのものを公開し、値をコピーしません。
未定義のexport、不正な名前や宣言、異なる束縛への同名importは位置付きのエラーです。
include-library-declarationsの循環も診断します。

## import setと束縛

| 構文 | 動作 |
|---|---|
| `(myapp util)`のようなライブラリ名 | そのライブラリの全exportを取り込む。myappとutilは名前要素の例 |
| `(only import-set identifier...)` | 指定した公開名だけを取り込む |
| `(except import-set identifier...)` | 指定した公開名を除外する |
| `(prefix import-set prefix-identifier)` | 全公開名の先頭に指定した名前を付ける |
| `(rename import-set (from to)...)` | 公開名を同時に変更する。2つの名前の交換も可能 |

only、exceptとrenameで存在しない公開名を指定するとエラーです。
名前の変換は元のexport表やオブジェクトを変更しません。
同じ束縛を複数回importすることはできますが、同じローカル名に異なる束縛を取り込めません。
importした束縛はdefine、define-syntaxやset!で変更できません。
その値がVectorなどの可変オブジェクトなら、vector-set!などによる内容の変更は可能です。
ライブラリ自身の手続きやマクロは、自分で定義した束縛を更新できます。
ローカルの引数やlet束縛で同名を使うことも可能です。
マクロは名前を変更してimportしても、定義元の非公開束縛とリテラルの束縛同一性を保持します。

```scheme
(import (only (scheme base) + =))
(= (+ 20 22) 42) ; => #t
```

```scheme
(import (prefix (only (scheme base) + =) math:))
(math:= (math:+ 20 22) 42) ; => #t
```

```scheme
(import (scheme base))
(define (answer) result)
(define result 42)
(answer) ; => 42
```

プログラムは1個以上のimport宣言に続けて定義や式を記述します。
`(import)`も宣言として認めますが、束縛は増えません。
importしていない構文や手続きは暗黙に利用できません。
プログラム本体の途中や関数の内側のimportもエラーです。
各Runのプログラム環境は新しく、同じRuntimeのライブラリ環境とVMは共有します。

## 配置と検索（D07-L）

Gaucheのインストール時に設定したライブラリディレクトリと、利用者が追加する検索パスの方式に合わせます。
内部Runtimeの標準14ライブラリはGo実装と公開束縛表としてバイナリへ組み込まれ、作業ディレクトリやスクリプトの置き場所に依存しません。
Gaucheにもコアを事前コンパイルする仕組みと、Schemeファイルをインストール先へ置く仕組みがあります。
[ビルドと配置の定義](https://github.com/shirok/Gauche/blob/master/src/Makefile.in)と[検索パスの初期化](https://github.com/shirok/Gauche/blob/master/src/load.c)を参照しています。

外部のSchemeライブラリは次の順に検索します。

1. 内部Runtimeへ明示指定したディレクトリ。
2. 環境変数GURI_LOAD_PATHのディレクトリ。
3. インストール時に設定した追加ライブラリ用のsiteディレクトリ。
4. インストール時に設定した同梱Schemeライブラリ用のディレクトリ。

環境変数はOSのパスリスト区切りに従い、Unixではコロン、Windowsではセミコロンで区切ります。
空の要素は無視し、重複するパス表記は最初の指定を使います。
検索パスはRuntimeの作成時に読み取り、セッション中はその設定を使います。
カレントディレクトリやlib/、スクリプトに隣接するディレクトリを暗黙には検索しません。
任意の配置を検索パスへ指定できるため、スクリプト・作業ディレクトリ・ライブラリの場所を一致させる必要はありません。

インストール用の配置はGaucheにならい、`<prefix>/share/guri/<version>/lib`と`<prefix>/share/guri/site/lib`を使う設計です。
ビルド時のGoリンカ-X設定で内部coreのstandardLibraryDirectoryとsiteLibraryDirectoryへ絶対パスを保持できます。
未設定の開発バイナリではこれらの外部ディレクトリは追加しませんが、バイナリ内の標準ライブラリは利用できます。
インストール手順とCLIの検索パス指定は#67で接続します。
配布バージョン番号はリリース時に確定し、この配置の設計で新たなバージョン番号を決定しません。

`(myapp util)`は各検索先のmyapp/util.sldに対応します。
examples/は既存のサンプル置き場として維持し、暗黙の検索先へ追加しません。
標準ライブラリは処理系内の公開束縛表から提供し、検索ファイルで上書きしません。

名前要素はASCIIの英数字、ハイフンとアンダースコアをそのまま使用し、それ以外のUTF-8バイトを%HHで表します。
数字だけのSymbolとOSの予約デバイス名は先頭のバイトもエスケープします。
空のSymbolは`%`で表します。
これにより整数の1とSymbolの`|1|`、`..`やスラッシュを含む名前を、同じパスや親ディレクトリの指定へ潰しません。
たとえば`(myapp |1|)`はmyapp/%31.sld、`(myapp 1)`はmyapp/1.sldです。
OSのファイル名の長さや大文字小文字の規則はファイルシステムに従います。

include、include-ciとinclude-library-declarationsの相対パスは、それを記述したファイルの場所を基準にします。
プログラム中のincludeも同じ規則を使います。
ファイルはUTF-8で読み、ファイル名・行・列を構文へ保持します。
複数のdefine-libraryを同じ.sldファイルに記載できますが、検索で要求したライブラリ名の定義が必要です。
検索やincludeのI/O失敗、異なる名前の定義しかないファイル、循環依存はエラーです。

## 初期化と継続

ライブラリはセッション内で1つの初期化と束縛を共有します。
依存先を先に初期化し、取り込んだライブラリの本体もプログラムより先に実行します。
ライブラリ本体は同じVMのバイトコードで動き、ASTを実行時に評価し直しません。
初期化が正常復帰すると、同じRuntimeの次のRunでも初期化済みの束縛を使用します。
登録した定義の展開・初期化のエラーや正常復帰しない脱出はキャッシュし、自動では再試行しません。

初期化中のcall/ccで捕捉した継続も、同じセッションのプログラムから明示的に再開できます。
その場合は捕捉した制御の続きと共有セルを使い、既に実行した副作用は巻き戻しません。
明示的な再開で初期化が正常復帰した場合は、初期化済みとして扱います。
この再開を、自動で初期化を最初からやり直す動作とは区別します。

## 接続を補った標準手続き

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(features)` | 0 | 現在のfeature identifierを要素に持つ新しいList。入力の変更なし |
| `(procedure? obj)` | 1、任意 | 手続き・継続・パラメータなら#t。それ以外は#f。変更なし |
| `(file-exists? filename)` | 1、String | ファイルが存在すれば#t、なければ#f。変更なし |
| `(delete-file filename)` | 1、String | ファイルを削除し、結果は未規定。渡したStringは変更しない |

featuresとcond-expandはguri、exact-closed、exact-complex、ieee-float、full-unicode、ratiosを公開します。
言語全体の準拠監査が未完了なので、r7rs featureはまだ広告しません。
`(library name)` requirementは登録済みまたは検索して定義を取得できるライブラリを調べ、本体を初期化しません。
未提供の標準ライブラリや失敗をキャッシュしたライブラリは利用不可として扱います。
featureの結果に応じた宣言や式を選び、未選択の節のファイルは読みません。

delete-fileの失敗と存在確認でのOSエラーはfile-error?を満たす例外を送出します。
filenameの型が違う場合は通常のerror-objectを送出します。
引数数の不一致は、既存VMと同じく自動raiseへ未接続です。

```scheme
(import (scheme base))
(and (procedure? +)
     (procedure? (make-parameter 1))
     (not (procedure? #f))) ; => #t
```

```scheme
(import (scheme base))
(cond-expand
  ((and (library (scheme char)) (not (library (scheme process-context)))) 42)
  (else 0)) ; => 42
```

```scheme
(import (scheme base) (scheme file))
(define p (open-output-file "library-example.tmp"))
(close-port p)
(define existed (file-exists? "library-example.tmp"))
(delete-file "library-example.tmp")
(and existed (not (file-exists? "library-example.tmp"))) ; => #t
```

最後の例は独立した一時ディレクトリで検証します。
独自17件とChibi由来62件は内部Runtimeで期待出力まで成功しています。
公開CLIは旧処理系のままなので、その適合性測定の17 fail／62 failをこの結果で置き換えません。

- [環境、eval、loadとREPL](https://github.com/0x6d61/guri/wiki/Evaluation-REPL): datumの評価、変更禁止と対話用の環境、loadの規則と継続、内部REPLの入出力。
