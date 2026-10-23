# R7RSのdatum Reader

#46で、標準の記法を`internal/scheme`の値へ読み取る`internal/reader`を実装しました。
このページは新しい内部Readerの対応範囲です。
現在のCLIと`Interpreter.Eval`はまだ旧Readerと評価器を使い、新Readerは#48の内部コンパイラへ接続済みですが、公開CLIの切替は後続の移行作業です。
現在使える言語の仕様は[言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification)を参照してください。

## 読み取れる記法

| 対象 | 記法と扱い |
|---|---|
| Boolean | `#t`、`#f`、`#true`、`#false`。大文字も受け付ける |
| ListとPair | `()`、`(a b)`、`(a b . c)`。可変Pairでdatumを組み立てる |
| Vector | `#(a b)`。固定長のVectorを生成する |
| Bytevector | `#u8(0 255 #e2.0)`。各要素に0〜255の正確な整数を要求する |
| Character | `#\a`、`#\space`、`#\x1f600`。標準の名前とUnicode scalar valueを扱う |
| String | UTF-8文字列。標準のエスケープ、hex escape、行継続を解釈する |
| Symbol | 標準の識別子と`|two words|`。縦棒内のエスケープや空の名前`||`も扱う |
| 略記 | `'x`、準引用、unquote、unquote-splicingを対応する2要素のListへ展開する |

`nil`、`true`、`false`、`:key`は普通のSymbolです。
`[]`と`{}`は予約文字としてエラーにします。
`b"abc"`をBytesにはせず、SymbolのbとStringのabcという2つのdatumとして読みます。
旧Guriの特別扱いを新Readerへ引き継ぎません。

この範囲は[R7RS第2章](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-4.html)と[§7.1.1、§7.1.2](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-9.html)に対応します。

## コメントと大文字小文字

行コメント、入れ子の`#| ... |#`、datum commentの`#;`を読み飛ばします。
`#;`は次のdatum全体を読み、そのdatumを返却結果へ含めません。
コメント内の構文も読み取るため、不正なdatumを無条件に受け付けることはありません。

`#!fold-case`と`#!no-fold-case`は同じReaderでの後続の読み取りへ作用します。
fold-caseは識別子と文字名にUnicodeの完全なcase foldingを適用します。
文字リテラルそのものには適用せず、`#\A`はAのままです。
例えばStraßeはstrasse、Σςはσσに折り畳みます。

単純な小文字化では完全なcase foldingを満たせないため、Go公式の`golang.org/x/text/cases` v0.43.0を使用します。
Go 1.26では、識別子の分類とcase foldingのデータ版はUnicode 15.0.0です。
データ版の一致をunitテストで確認します。
版とチェックサムはgo.modとgo.sum、再配布に必要な表示は[外部ライセンス表示](https://github.com/0x6d61/guri/blob/master/THIRD_PARTY_NOTICES.md)で管理します。

## 文字列と位置情報

入力はUTF-8とし、不正なバイトはコメント内にあっても診断します。
Stringでは`\a`、`\b`、`\t`、`\n`、`\r`、`\"`、`\\`、`\|`と`\x...;`を解釈します。
不明なエスケープはReaderErrorとする方針です。
rawなCRLFとCRはString内ではLFへ正規化し、バックスラッシュによる行継続とその周囲の空白を取り除きます。

各Nodeはdatumとソース位置を持ち、PairのCar/CdrとVectorの各要素にも位置を残します。
位置はファイル名、1始まりの行と列で、列はUnicode scalar単位です。
CRLFは1回の改行として数えます。

Readerが返すdatumは変更可能です。
プログラムのquoteや自己評価するコレクションを定数として扱う処理は、後続の評価器で適用します。
Reader自体で値を凍結し、標準のreadが返すデータまで変更不能にすることはありません。

## 数値の読み取り

`scheme.ParseNumber`を共通の数値変換として実装し、新Readerから使います。
#54のstring->numberも同じ変換へ接続する予定です。

- `#b`、`#o`、`#d`、`#x`による基数指定。
- `#e`、`#i`による正確性指定。基数との指定順はどちらも扱う。
- 任意精度の整数、有理数、10進小数と指数。
- 直交形式の複素数と極形式。
- 正負の無限大、NaN、負のゼロ。

例えば`#e1.25`は5/4、`#xFF`は255、`1/2-3/4i`は正確な複素数になります。
数値を読めない場合に、数字や予約文字から始まる不正なトークンをSymbolへ置き換えません。
`+`や`.foo`など、標準の識別子として有効なものはSymbolとして扱います。

正確な数値の小数と指数の処理にはmath/big.Ratを使用します。
この変換が表現できない入力は、実装上の範囲外として診断します。
極形式は通常binary64で計算します。
`#e2@0`や`#e0@1`のように正確な結果を保持できる場合は受け付けますが、正確な極形式で非ゼロの角度から一般の正確な実部と虚部を作る処理は実装していません。
例えば`#e1@1`は表現できない数値として診断します。
無限大やNaNを正確な数として読むこともできません。
これらは[R7RS §6.2.3](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html#TAG:__tex2page_sec_6.2.3)の実装制限として区別します。

数値の読み取りに対応したことは、算術や標準の数値述語が言語から使えることを意味しません。
それらは#54で接続します。

## 内部APIの使用例

`ReadAll`はソース全体を読み、`New`と`Read`は同じソースを1つずつ読みます。
`Read`は入力終了時にGoのio.EOFを返し、GoのnilをScheme値にはしません。
内部APIであり、現在の公開Go APIは変更していません。

次の例は`internal/reader/example_test.go`で出力を検証します。

```go
nodes, err := reader.ReadAll("example.scm", "(name . #u8(1 2)) #e1.25", nil)
if err != nil {
    panic(err)
}
fmt.Println(nodes[0].Car.Datum().(*scheme.Symbol).Name())
fmt.Println(nodes[0].Cdr.Datum().(*scheme.Bytevector).Bytes())
fmt.Println(nodes[1].Datum().(*scheme.Rational).String())
// name
// [1 2]
// 5/4
```

## 検証と残る作業

独自作成17ケースとChibi由来62ケースの全79プログラムを、新Readerで最後まで読めることをunitテストで検証します。
この検査は構文の読み取りであり、プログラムの実行成功とは別です。
現行CLIによる79件の結合測定は、引き続き未対応のfailとして保持します。

datum labelの`#n=`と`#n#`による共有と循環は#58の作業です。
現在はErrDatumLabelsを原因として返し、Symbolや別の値に置き換えません。
datum comment内のlabelも、同じ未対応範囲に含みます。
このReaderをR7RS全体の準拠完了として扱いません。
