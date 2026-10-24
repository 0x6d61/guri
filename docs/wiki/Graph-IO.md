# datum labelと共有・循環データの読み書き

このページは内部Reader、Writerとコンパイラへ接続したグラフの契約を説明します。
公開CLIと公開Go APIは従来のGuriのままです。
ポートを取るread、write、write-sharedとwrite-simpleの標準束縛は#60で内部NewCoreへ接続済みです。
本ページのScheme例は内部NewCoreで、Go例は内部パッケージのunitテストで検証します。
参照先は[R7RS-small §2.4](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-4.html#TAG:__tex2page_sec_2.4)と[§6.13.3](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.13.3)です。

## ラベルの構文と範囲

`#n=datum`はdatumを返し、nで参照できるようにします。
`#n#`は先に定義した同じオブジェクトを返します。
nは10進数字の列で、機械整数の幅による上限は設けません。
先頭のゼロを除いて番号を照合します。

有効範囲は最も外側のdatum内で定義位置より後ろです。
Reader.Readを呼ぶごとに別の範囲になり、前のReadのラベルを参照できません。
未定義や前方参照、対象のオブジェクトが存在しない直接の自己参照はReaderErrorです。
例えば`#0=#0#`はエラーですが、`#0=(a . #0#)`はPairを作れるため受け付けます。
不正な入力にはファイル名、行と列を保持します。

ラベルはPairとVectorだけでなく、String、Bytevectorや数値などのdatumにも使えます。
同じ可変オブジェクトへの参照を返し、コピーしません。
リストのcdrは任意なので、非正規リストと循環も表せます。
同一範囲で同じ番号を再び定義した場合、このReaderは以後の参照に新しい定義を使い、既に読み取った参照先は変えません。
この再定義の扱いを移植可能な必須結果として使わず、Writerは番号を再定義しません。

```scheme
(define x '(#0=(a b) #0#))
(eq? (car x) (cadr x))                ; => #t
(define cycle '#0=(a . #0#))
(eq? cycle (cdr cycle))               ; => #t
(define v '#0=#(#0#))
(eq? v (vector-ref v 0))              ; => #t
```

## 読取時のグラフ構築とコメント

新しいScheme型を追加せず、既存の可変Pair、VectorとBytevectorを使います。
ReaderのNode内に一時的な参照を保持し、全ての参照先を決めてから各オブジェクトの辺を接続します。
参照ごとのNodeには、その参照が現れたソース位置を残します。
Nodeの辺とDatumの辺は解決後に一致します。
ラベルのための一時値を言語の値として公開しません。

Reader自身はFreezeLiteralを適用せず、readで使うための可変データを返します。
コンパイラが定数として使うときに、到達する全てのリテラルを不変にします。
循環したリテラルも、深いコピーを作らず同一性を保ちます。

`#;`で捨てるdatumも構文とラベルを検証します。
捨てたdatum内のラベル定義を後続のコードへ漏らしません。
コメント内の新しいラベル範囲は、外側で既に見える定義を参照できます。
Bytevector要素の型と0〜255の検査はラベル解決後に行います。

## 実行コード、準引用とマクロ

quoteと自己評価するVectorなどのリテラルには循環を含められます。
実行する式の循環と、準引用の循環はSyntaxErrorにします。
リストの形だけを調べる処理も循環を検出し、終端を探して反復し続けません。
quoteの判定は名前の文字列だけでなく束縛を使い、ローカル変数で隠したquoteをリテラル構文と誤認しません。

マクロのテンプレートは、datumの同一性、繰り返し位置とellipsisの有効状態をキーとして構築します。
繰り返しでリストの長さが変わっても、参照先を決めてから辺を接続するため共有tailを失いません。
パターン変数の置換、引用の共有と循環、繰り返しの展開を組み合わせて扱います。
入力から捕捉したNodeを書き換えず、元の文脈とソース位置を保持します。
展開後にオブジェクトを作れない参照だけの循環が残った場合は診断します。

```scheme
(define-syntax make-data
  (syntax-rules ()
    ((_ x ...) (quote (x ... #0=(a) #0#)))))
(define data (make-data 1 2))
(eq? (list-ref data 2) (list-ref data 3)) ; => #t

(define-syntax make-ring
  (syntax-rules ()
    ((_ x ...) (quote #0=(x ... . #0#)))))
(define ring (make-ring 1 2))
(eq? ring (cddr ring))                    ; => #t
```

## Writerの3方式

内部の`writer.Write(ctx, output, value, mode)`はio.Writerへ出力し、成功ならnil、失敗ならエラーを返します。
入力を変更せず、参照先のオブジェクトや出力先を閉じません。
次の表の標準手続き名は対応予定の意味であり、現時点のScheme束縛ではありません。
標準の呼び出し形式はどれも`(手続き obj [port])`の1または2引数で、任意の値とテキスト出力ポートを取り、返り値は未規定です。

| mode | 対応する手続き | 共有と循環の出力 |
|---|---|---|
| Normal | write | 循環がある場合だけdatum labelを使う。循環に属するPairとVectorをラベルで表す |
| Shared | write-shared | 複数箇所から参照するPairとVectorの共有を表す。この実装は共有StringとBytevectorも保持する |
| Simple | write-simple | ラベルを使わず共有部分を繰り返し出力する |

Normalは循環しない共有部分を別々に出力するため、その同一性の往復を保証しません。
Sharedは同じオブジェクトを定義と参照で表し、読み戻したグラフでも共有を保持します。
番号の具体的な値は外部プログラムの判定条件に使いません。
Simpleは循環で終了しないことがあります。
内部APIでは渡したcontextのキャンセルを検査するため、必要に応じて出力を中断できます。
これを循環データに対する新しいSchemeエラー条件とは扱いません。

```go
nodes, err := reader.ReadAll("data.scm", `(#0=(a b) #0#)`, nil)
if err != nil {
    panic(err)
}
for _, mode := range []writer.Mode{writer.Normal, writer.Shared, writer.Simple} {
    var output strings.Builder
    if err := writer.Write(context.Background(), &output, nodes[0].Datum(), mode); err != nil {
        panic(err)
    }
    fmt.Println(output.String())
}
// ((a b) (a b))
// (#0=(a b) #0#)
// ((a b) (a b))
```

## 外部表現、エラーと検証

Pair、Vector、Bytevector、String、Symbol、Character、数値、Booleanと空リストを標準の外部表現で出力します。
Stringは引用とエスケープを付け、非ASCIIのSymbolは縦線で囲みます。
数値には数値ライブラリとReaderに共通する往復可能な表記を使います。
読み戻せるdatum表現がないProcedure、Port、Recordなどは不透明な型表示にし、その読み戻しを保証しません。
`#<cycle>`を標準のグラフ表現の代用にしません。

有限グラフの解析と出力は明示的な作業リストを使い、Goの再帰呼び出しに深さを依存させません。
循環部分は強連結成分を調べて識別し、Sharedでは各オブジェクトへの参照数を使います。
出力先のエラーと短い書き込みを呼び出し元へ返します。
キャンセル後は後続の出力を行いません。

Reader、Writerとコンパイラで、共有、循環、スコープと診断、マクロのellipsisを検証します。
Writerでは10万要素のリストと50通りの有限グラフを往復し、Sharedの内容と同一性を確認します。
既存の公開CLIの適合測定と内部のunitテストは区別し、read/writeの標準束縛とポート連携は#59/#60で内部VMへ接続しました。

ストリーム入力、displayとファイル操作は[datumとファイルの入出力](https://github.com/0x6d61/guri/wiki/Datum-File-IO)を参照してください。
