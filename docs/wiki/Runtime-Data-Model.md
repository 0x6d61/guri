# R7RSの内部データモデル

#45で、Readerと評価器が共有する値層を`internal/scheme`へ実装しました。
このページは内部の契約とunitテストの対象を説明します。
現在のCLIや`Interpreter.Eval`はまだ旧Guriの値を使い、ここに記載した数値型や標準手続きを言語から使える状態にはなっていません。
言語側の接続は#46以降で進めます。

## 採用した方針

通常の内部表現は実装側で選ぶ方針とし、同じ内容の確認を繰り返しません。
数値はユーザーの承認により、任意精度整数、正確な有理数、binary64の不正確な実数、正確または不正確な複素数に対応します。
任意精度浮動小数点は対象に含めません。
利用者から見える新たな制限や公開APIへ大きな影響が生じる場合は相談します。

## 値と同一性

`Value`は内部パッケージで定めた型だけが実装するインターフェースです。
旧GuriのMap、Keyword、Nil、不変Bytes、Go Slice型Vectorは含めません。
GoのnilはSchemeの値ではなく、引数に渡すと内部のプログラミングエラーとして扱います。
空リスト、EOF、未規定の結果、初期化前の束縛、ゼロ個の返り値を区別します。

| 対象 | 表現と検証した振る舞い |
|---|---|
| Boolean | falseとtrueを分離し、条件ではfalseだけを偽にする |
| 空リスト | 専用の単一値Null。PairやBooleanとは別の型で、真として扱う |
| Pair | carとcdrを持つ可変オブジェクト。非正規リスト、共有、循環を表現する |
| Vector | 固定長の可変オブジェクト。代入は同じオブジェクトを指し、Copyは独立した要素領域への浅いコピー |
| Bytevector | 固定長の可変バイト列。更新値を0〜255に制限し、外部へ内部配列を公開しない |
| Character | Unicode scalar value。サロゲートや範囲外の値を拒否する |
| String | 固定長の文字列をrune配列で保持する。文字位置の更新でUTF-8のバイト幅が変わっても文字数は変わらない |
| Symbol | 名前を不変にし、処理系ごとのSymbolTableで同じ名前を共有する |
| Record | 型の生成ごとに異なるRecordTypeを持ち、フィールドごとの変更可否を保持する |
| Procedure | オブジェクトの同一性とCodeへの参照を保持する。呼び出し処理は評価器で実装する |
| Port | オブジェクトの同一性とResourceへの参照を保持する。読み書きと借用・所有、開閉は内部portsが管理する。詳細は[ポート](https://github.com/0x6d61/guri/wiki/Ports) |

コンストラクタとコピー用のアクセサは、外部のGo Sliceやmath/bigの可変領域をそのまま共有しません。
Schemeのオブジェクトを代入するときの共有とは区別します。
Vectorの浅いコピーでは、内部のPairなどへの参照は共有します。
StringとBytevectorのコピーは、新しい領域に文字またはバイトをコピーします。
空のコレクションも各生成で独立したオブジェクトを持ちます。

これらの位置と型の要件は[R7RS §3.2と§3.4](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-5.html)に対応します。
Schemeのeq?、eqv?、equal?は#55で内部VMへ接続しました。
[Pair、List、Symbolと等価性](https://github.com/0x6d61/guri/wiki/Pairs-Lists-Symbols)に共有、浅いコピー、循環比較の契約を記載します。
公開CLIは未接続で、importは[内部Runtime](https://github.com/0x6d61/guri/wiki/Libraries)へ接続済みです。

## リテラルの変更禁止

`FreezeLiteral`は、リテラルから到達するPair、Vector、String、Bytevectorを変更不可にします。
共有と循環はそのまま保持し、明示的な作業リストで走査するため循環しても終了します。
コピーしたコンテナは変更できますが、浅いコピーで共有した内部の定数までは変更可能にしません。

変更不可、添字の範囲外、不正なu8値、不正なUTF-8は、内部の操作から判別可能なGoエラーとして返します。
Schemeの例外への変換、エラー述語とハンドラの処理は#53以降で接続します。

## 数値の正確性と不変性

| 表現 | 契約 |
|---|---|
| Integer | math/big.Intで任意精度を保持し、入力と返却用のBigIntをコピーする |
| Rational | math/big.Ratで約分し、分母を正にする。分母1の値はIntegerへ正規化する |
| Float | float64を保持し、NaN、正負の無限大、負のゼロを保存する |
| Complex | 正確な実部と虚部、またはbinary64の実部と虚部を保持する。片方が不正確なら両方をbinary64へ変換する |

虚部が正確なゼロなら、実部の正確性にかかわらず実数へ正規化します。
不正確な虚部のゼロは符号を含めて保持し、複素関数で必要になる情報を落としません。
`Real`というGoインターフェースは複素数の成分の内部表現用であり、標準のreal?述語の実装ではありません。

これらは値を保持する処理です。
Readerは#46、算術演算、比較、書式と数値述語は#54で内部環境へ接続しました。
詳細は[標準数値手続き](https://github.com/0x6d61/guri/wiki/Numbers)を参照してください。
公開CLIの切替は後続作業です。
標準ライブラリのimportは[内部Runtime](https://github.com/0x6d61/guri/wiki/Libraries)へ接続済みです。
正確な型の値が保持できることと、言語の`(/ 1 3)`が動くことを区別します。

次は言語のコードではなく、値層のGo使用例です。
`internal/scheme/example_test.go`のExampleで出力を検証します。

```go
value, err := scheme.NewRational(big.NewInt(2), big.NewInt(6))
if err != nil {
    panic(err)
}
fmt.Println(value.(*scheme.Rational).String(), value.Exact())
// 1/3 true
```

Stringでは、次の更新後も文字数は3です。

```go
value, err := scheme.NewString("A日本")
if err != nil {
    panic(err)
}
ch, err := scheme.NewCharacter('😀')
if err != nil {
    panic(err)
}
if err := value.Set(1, ch); err != nil {
    panic(err)
}
fmt.Println(value.Text(), value.Len())
// A😀本 3
```

## 束縛と構文の契約

Cellは値を保存する場所です。
クロージャはCellへの参照を保持し、再代入を共有します。
初期化前のCellを読むとErrUninitializedを返し、Nullや未規定の結果には置き換えません。

IdentifierはSymbolと変更不可のスコープ集合を持ちます。
スコープの追加は新しい文脈を返し、元の識別子を変えません。
BindingIDは同じ名前でも異なる束縛を区別するための識別子です。
SameContextによる文脈の比較は、束縛解決そのものではありません。
束縛解決と衛生的展開は#48と#49で実装します。

Syntaxはdatum、ファイル名、行、列とスコープを保持する内部のメタデータです。
行と列は1始まりで、列はUnicode scalar単位です。
コンテナ内のソース位置の木構造と走査はReaderで組み立てます。
SyntaxやCellはSchemeの値として渡しません。

## 制御と資源の境界

Resultsは0個以上の返り値を保持し、ListやVectorなどのSchemeの値とは区別します。
1個を要求する箇所では個数を検証し、0個をNullやGoのnilへ変換しません。
Raisedは任意のScheme値、継続可能かどうか、ソース位置を保持します。
例外のpayloadを特定のエラーオブジェクトに限定しません。

ProcedureCodeとPortResourceは、評価器やポート実装が保持する内容への参照です。
ProcedureCodeに直接呼び出すGoコールバックを要求せず、#47で継続を扱える方式を妨げないようにします。
この値層は継続の実装方式、Portの開閉、入出力やホストへの終了伝達を実装しません。
現在の公開Go APIは変更せず、これらの契約はまずinternalパッケージ内で使います。

Go GCでオブジェクト、束縛、コードや資源の参照を管理します。
到達可能な循環や共有、Procedureが保持するCellがGC実行後も参照できることをunitテストで確認しています。
到達不能な循環の回収時期や、資源をfinalizerだけで閉じる契約は設けません。

## 後続への接続

新しい値層と旧Guriの値との自動変換や互換モードは追加しません。
#46で標準のReaderを接続し、#47と#48で新しい評価器とコア構文へ移行します。
その際に旧Map、Keyword、Nil、不変BytesとSliceヘッダ型Vectorの使用箇所を取り除き、#67で残存を監査します。

今回の値層のunitテストが通っても、独自作成17件とChibi由来62件の結合テストが通ったことにはなりません。
要求台帳の下位IDに値層の検証範囲を記録し、言語での検証は後続PRの証拠として追加します。

#56でCharacter/Stringの標準手続きを内部VMへ接続しました。
Stringの重複copyとfillは空範囲でも不変性を検査し、不正な範囲を変更前に診断します。
[CharacterとString](https://github.com/0x6d61/guri/wiki/Characters-Strings)に型、範囲、共有とUnicodeの契約を記載します。

#57で固定長VectorとBytevectorの標準操作を内部VMへ接続しました。
bulk copy/fillは格納領域を伸ばさず、不変性と全範囲を変更前に検証します。
[VectorとBytevector](https://github.com/0x6d61/guri/wiki/Vectors-Bytevectors)に浅いコピー、共有、UTF-8と継続の契約を記載します。

#62でdefine-record-typeを既存Record/RecordTypeへ接続しました。
フィールドの識別子はコンパイラで文脈を判定し、内部のタグと宣言順の位置へ対応付けます。
[Record型定義](https://github.com/0x6d61/guri/wiki/Records)に型の生成、フィールドの初期化と可変性を記載します。

## 遅延評価の内部表現

#63のPromiseは非公開のRecordで、転送先・計算済みかどうか・値またはクロージャ・評価モードを保持します。
Schemeへ独自のValue型やアクセス手続きを追加しません。
forceは共有する根を探し、delay-forceの次のPromiseと状態を共有して古いクロージャを解放します。
転送先は経路短縮し、例外で中断した既存の別名も同じメモ化結果を参照します。
Promiseを手続きとして呼ぶことや暗黙のforceは提供しません。
[Promiseと遅延評価](https://github.com/0x6d61/guri/wiki/Lazy-Evaluation)に使用例とエラーを記載します。

## 環境指定子の内部表現

#64の環境指定子は非公開Recordで保持します。
非公開のハンドルから所有Runtimeとコンパイラ環境を参照し、通常のScheme手続きとして呼び出せないようにします。
セッション全体の保持表へ登録せず、参照されない環境はGCの対象になります。
変更禁止の環境はライブラリのセルを共有し、対話環境の初期標準束縛には別のセルを用意します。
標準束縛の別名は同じ対話用セルを参照し、可変オブジェクトの内容は浅く共有します。
