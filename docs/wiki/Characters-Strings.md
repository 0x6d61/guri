# SchemeのCharacterとString

このページの50手続きは内部の`core.NewCore`環境に登録しています。
#67で公開CLIへ接続しました。旧ルートGo APIは削除し、新しい公開Go APIは提供しません。
標準ライブラリのimportは内部Runtimeで利用でき、[提供範囲](https://github.com/0x6d61/guri/wiki/Libraries)を区別します。
従来のnth、sliceとlenを標準の名前として追加しません。
base、charとr5rsへの公開束縛の割り当ては`docs/r7rs/exports.csv`で追跡します。
参照先は[R7RS-small §6.6、§6.7、§6.10](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html)です。

## 文字集合、索引と共通規則

CharacterはUnicode scalar valueです。
U+0000からU+D7FF、U+E000からU+10FFFFまでを扱い、サロゲートを含めません。
文字の分類と大小変換には、既存のD02に従ってGo 1.26とx/textのUnicode 15.0.0を使います。
文字列内でNULを含むスカラー値を禁止しません。

Stringは文字位置が固定された可変配列で、長さと添字はコードポイント単位です。
結合文字列の見た目の文字数、UTF-8バイト数とは異なります。
string-refは1文字StringではなくCharacterを返します。
文字のUTF-8幅が変わる更新でも、Stringの長さは変わりません。
Characterと整数は不変で、代入でStringをコピーせず、同じ可変領域を参照します。

以下の`k`、`start`、`end`と`at`は非負の正確な整数です。
`[arg]`は省略可能、`...`は繰り返しを示す説明用の表記です。
範囲は`start <= end <= length`で、startを含みendを含みません。
省略時のstartは0、endは対象の長さです。
表に変更を記載しない手続きは引数を変更しません。
引数型、範囲外、対応しないCharacter値と変更禁止のエラーはerror-objectを送出し、guardで捕捉できます。
引数数の不一致はVMの引数数エラーであり、自動送出にはまだ接続していません。

```scheme
(string-length "Aλ😀")                     ; => 3
(char->integer (string-ref "Aλ😀" 2))       ; => 128512
(char=? (integer->char #x3bb) #\λ)           ; => #t
```

## Characterの手続き

| 手続き | 引数数・型 | 返り値 |
|---|---|---|
| `(char? obj)` | 1、任意 | Characterなら#t |
| `(char=? c1 c2 c...)`、`(char<? c1 c2 c...)`、`(char<=? c1 c2 c...)`、`(char>? c1 c2 c...)`、`(char>=? c1 c2 c...)` | 2以上、Character | コードポイントの等号／狭義昇順／昇順／狭義降順／降順を満たせば#t |
| `(char-ci=? c1 c2 c...)`、`(char-ci<? c1 c2 c...)`、`(char-ci<=? c1 c2 c...)`、`(char-ci>? c1 c2 c...)`、`(char-ci>=? c1 c2 c...)` | 2以上、Character | char-foldcaseの結果を比較したBoolean |
| `(char-alphabetic? c)` | 1、Character | Unicode Alphabetic属性なら#t |
| `(char-numeric? c)` | 1、Character | Numeric_Type=Decimalなら#t |
| `(char-whitespace? c)` | 1、Character | White_Space属性なら#t |
| `(char-upper-case? c)`、`(char-lower-case? c)` | 1、Character | Uppercase／Lowercase属性なら#t |
| `(digit-value c)` | 1、Character | 10進数字の0〜9、数字以外は#f |
| `(char->integer c)` | 1、Character | スカラー値の正確な整数 |
| `(integer->char k)` | 1、正確な整数 | 対応するCharacter |
| `(char-upcase c)`、`(char-downcase c)` | 1、Character | 言語に依存しない単純な大文字／小文字変換のCharacter |
| `(char-foldcase c)` | 1、Character | Unicodeの単純case foldingのCharacter |

分類はASCIIの範囲に限定せず、Alphabeticなどの派生属性も含めます。
char-numeric?はローマ数字や上付き数字を10進数字として扱いません。
integer->charは負の値、サロゲートとU+10FFFFを超える値でエラーです。
数値として整数であってもFloatは受け付けません。
char-foldcaseは1文字を返し、ßを2文字のssへ展開しません。
言語固有のTurkic foldingは使いません。

```scheme
(char-ci=? #\Σ #\ς #\σ)       ; => #t
(char-alphabetic? #\x0345)     ; => #t
(digit-value #\٤)              ; => 4
(digit-value #\𝟡)              ; => 9
(char=? (char-foldcase #\ẞ) #\ß) ; => #t
```

## Stringの生成、取得とコピー

| 手続き | 引数数・型 | 返り値と共有 |
|---|---|---|
| `(string? obj)` | 1、任意 | Stringなら#t |
| `(make-string k [c])` | 1または2、kとCharacter | k文字の新しい可変String。c省略時の内容は未規定 |
| `(string c...)` | 0以上、Character | 引数を格納する新しい可変String |
| `(string-length s)` | 1、String | 文字数の正確な整数 |
| `(string-ref s k)` | 2、Stringとk | k番目のCharacter |
| `(substring s start end)` | 3、Stringと境界 | 指定範囲をコピーした新しい可変String |
| `(string-copy s [start [end]])` | 1〜3、Stringと境界 | 同じく独立した新しい可変String |
| `(string-append s...)` | 0以上、String | 連結した新しい可変String。空なら空String |
| `(string->list s [start [end]])` | 1〜3、Stringと境界 | Characterを格納する新しい可変List |
| `(list->string list)` | 1、Characterのproper List | 新しい可変String |

コピー、部分文字列、連結とList変換は、元のStringの可変領域を共有しません。
string-refの添字は長さより小さい必要があります。
空の部分範囲と、長さに等しいendは受け付けます。
list->stringは循環List、improper ListとCharacter以外の要素でエラーです。
確保する長さにはGoのアドレス空間と利用可能メモリの制約があります。

```scheme
(substring "Aλ😀" 1 3)                   ; => "λ😀"
(string-copy "Aλ😀" 1 2)                 ; => "λ"
(list->string (string->list "aλ😀" 1 3)) ; => "λ😀"
(string-append "a" "λ" "😀")            ; => "aλ😀"
```

## Stringの更新

| 手続き | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(string-set! s k c)` | 3、String、k、Character | 元のk番目を変更。結果は未規定 |
| `(string-copy! to at from [start [end]])` | 3〜5、String、位置、String、境界 | 元のtoのatから、fromの範囲をコピー。結果は未規定 |
| `(string-fill! s c [start [end]])` | 2〜4、String、Characterと境界 | 元の指定範囲を同じCharacterに変更。結果は未規定 |

string-copy!は範囲を全て検証してから書き込むため、不正な範囲で一部分だけ変更しません。
同じStringの範囲が重なっていても、一時コピーを介した場合と同じ結果になります。
コピー元が不変のStringでも受け付けます。
リテラルとsymbol->stringが返すStringは変更できません。
書き込む範囲が空であっても、不変Stringを変更対象に渡すとエラーです。

```scheme
(define s (string-copy "abcde"))
(string-copy! s 1 s 0 4)
s                                           ; => "aabcd"
(define x (string-copy "abc"))
(string-set! x 1 #\😀)
x                                           ; => "a😀c"
(guard (e (else #t)) (string-set! "a" 0 #\x) #f) ; => #t
```

## 比較と完全な大小変換

| 手続き | 引数数・型 | 返り値 |
|---|---|---|
| `(string=? s1 s2 s...)`、`(string<? s1 s2 s...)`、`(string<=? s1 s2 s...)`、`(string>? s1 s2 s...)`、`(string>=? s1 s2 s...)` | 2以上、String | コードポイントの辞書順による比較のBoolean |
| `(string-ci=? s1 s2 s...)`、`(string-ci<? s1 s2 s...)`、`(string-ci<=? s1 s2 s...)`、`(string-ci>? s1 s2 s...)`、`(string-ci>=? s1 s2 s...)` | 2以上、String | string-foldcaseした値の比較のBoolean |
| `(string-upcase s)`、`(string-downcase s)`、`(string-foldcase s)` | 1、String | 完全な大文字／小文字変換、またはcase foldingの新しい可変String |

共通の接頭部分しかなければ短いStringを辞書順で先に置きます。
大小変換で元のStringを変更せず、結果の領域も共有しません。
文字列の変換は文字数が増える場合や、前後の文字に依存する場合があります。
Unicode正規化やロケール固有の変換は行いません。

```scheme
(string-ci=? "Straße" "STRASSE") ; => #t
(string-upcase "straße")         ; => "STRASSE"
(string-downcase "ΟΣ")           ; => "ος"
(string-foldcase "Straße Σς")    ; => "strasse σσ"
(string-foldcase "Ꭰꭰ")           ; => "ᎠᎠ"
```

## string-mapとstring-for-each

| 手続き | 引数数・型 | 返り値と副作用 |
|---|---|---|
| `(string-map proc s s...)` | 2以上、手続きと1個以上のString | 対応するCharacterをprocへ渡し、返したCharacterを格納する新しい可変String |
| `(string-for-each proc s s...)` | 2以上、同じ型 | 先頭から順にprocを呼ぶ。結果は未規定 |

最も短いStringの長さで停止します。
procの引数数はStringの個数と同じです。
string-mapのprocは単一のCharacterを返す必要があり、それ以外はエラーです。
string-for-eachではprocの結果を捨てます。
procが入力Stringを変更するのは規格上エラーです。
string-mapの適用順は規格上未規定ですが、この実装は先頭から呼びます。
結果は入力Stringと可変領域を共有しません。

procを同じ動的環境でVMのInvoke/Resume契約により呼びます。
途中で保存した継続を複数回再開でき、以前返したStringは変更しません。
空の入力ではprocを呼びません。

```scheme
(string-map char-upcase "aλ")                    ; => "AΛ"
(string-map (lambda (a b) b) "long" "λ😀")        ; => "λ😀"
(define total 0)
(string-for-each (lambda (c) (set! total (+ total (char->integer c)))) "ab")
total                                            ; => 195
```

## Unicodeデータと検証

Readerのfold-case、char-foldcaseとstring-foldcaseは共通の`internal/textcase`を使います。
x/text v0.43.0のGo 1.26用テーブルはCherokeeの大文字86文字を誤って小文字にfoldするため、この文字集合の結果を規定の大文字に補正します。
Readerと文字手続きの回帰テストで、大小のCherokeeを同じ正規の大文字へ変換することと冪等性を確認します。

開発時に[Unicode 15.0.0 CaseFolding.txt](https://www.unicode.org/Public/15.0.0/ucd/CaseFolding.txt)を照合し、全スカラー値でC/Sの1,454対応、C/Fの1,530対応と既定の恒等変換が一致することを確認しました。
照合した原本のSHA-256は`cdd49e55eae3bbf1f0a3f6580c974a0263cb86a6a08daa10fbf705b4808a56f7`です。
この外部原本をリポジトリに取り込まず、通常のunitテストは外部通信に依存しません。
Goまたはx/textのUnicode版を更新するときは、版の固定テストを更新する前にこの照合をやり直します。

`internal/core/text_test.go`で使用例、境界、分類、コピー、継続と2万文字の反復を検証します。
`internal/scheme`で重複コピーと空の不変Stringへの変更を検証し、`internal/textcase`とReaderでfoldingの回帰を確認します。
公開CLIの17件とChibi由来62件の適合測定は内部実装の検証と別に記録します。
