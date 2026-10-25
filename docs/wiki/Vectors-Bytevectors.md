# SchemeのVectorとBytevector

このページの27手続きは内部の`core.NewCore`環境に登録しています。
#67で公開CLIへ接続しました。旧ルートGo APIは削除し、新しい公開Go APIは提供しません。
標準ライブラリのimportは内部Runtimeで利用でき、[提供範囲](https://github.com/0x6d61/guri/wiki/Libraries)を区別します。
従来のGo Slice型Vectorや不変Bytesの仕様とは区別します。
baseとr5rsへの公開束縛の割り当ては`docs/r7rs/exports.csv`で追跡します。
参照先は[R7RS-small §6.8、§6.9、§6.10](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-8.html)です。

## 長さ、参照と共通規則

Vectorは任意の値を格納する固定長の可変オブジェクトです。
Bytevectorは0〜255の正確な整数に対応するバイトを格納する固定長の可変オブジェクトです。
長さは生成後に変わらず、容量や配列を共有するsliceの標準手続きはありません。
代入は同じオブジェクトを参照し、コピーしません。
vector-appendとbytevector-appendは入力を伸ばさず、新しいオブジェクトを返します。

以下の`k`、`start`、`end`と`at`は非負の正確な整数です。
`byte`は0〜255の正確な整数で、`1.0`は受け付けません。
`[arg]`は省略可能、`...`は繰り返しを示す説明用の表記です。
範囲は`start <= end <= length`で、startを含みendを含みません。
省略時のstartは0、endは対象の長さです。
表に変更を記載しない手続きは入力を変更しません。
不正な型、添字、バイト値、UTF-8と変更禁止はerror-objectを送出し、guardで捕捉できます。
引数数の不一致はVMの引数数エラーであり、自動送出にはまだ接続していません。
確保できる長さにはGoのアドレス空間と利用可能メモリの制約があります。

## Vectorの生成と取得

| 手続き | 引数数・型 | 返り値 |
|---|---|---|
| `(vector? obj)` | 1、任意 | Vectorなら#t |
| `(make-vector k [fill])` | 1または2、kと任意の値 | k要素の新しい可変Vector。fill省略時の内容は未規定 |
| `(vector obj...)` | 0以上、任意 | 引数を格納する新しい可変Vector |
| `(vector-length v)` | 1、Vector | 長さの正確な整数 |
| `(vector-ref v k)` | 2、Vectorとk | 格納した値そのもの |
| `(vector-copy v [start [end]])` | 1〜3、Vectorと境界 | 指定範囲を浅くコピーした新しい可変Vector |
| `(vector-append v...)` | 0以上、Vector | 連結した新しい可変Vector。空なら空Vector |

make-vectorのfillは各要素で同じ参照を共有します。
コピーや連結は格納領域を独立させますが、中のPair、Vectorなどは浅く共有します。
vector-refの添字は長さより小さい必要があります。

```scheme
(vector 1 'a #t)                     ; => #(1 a #t)
(make-vector 3 1)                    ; => #(1 1 1)
(vector-ref '#(1 2 3) 1)             ; => 2
(vector-copy '#(1 2 3) 1 3)          ; => #(2 3)
(vector-append '#(1) '#(2 3))        ; => #(1 2 3)
```

## Vectorの変更と共有

| 手続き | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(vector-set! v k obj)` | 3、Vector、kと任意 | 元のk番目の値を変更。結果は未規定 |
| `(vector-copy! to at from [start [end]])` | 3〜5、Vector、位置、Vectorと境界 | 元のtoへ指定範囲を浅くコピー。結果は未規定 |
| `(vector-fill! v fill [start [end]])` | 2〜4、Vector、任意の値と境界 | 元の範囲をfillの同じ参照で埋める。結果は未規定 |

copy!は範囲を全て検証してから書き込みます。
同じオブジェクトの範囲が重なっていても、一時コピーを介した場合と同じ結果になります。
コピー先の長さは変わらず、残りの領域が足りなければエラーです。
リテラルを変更対象にするのはエラーで、空の変更範囲でも禁止します。
リテラルからコピーした新しいVectorは変更できますが、浅く共有した内部のリテラルは不変のままです。

```scheme
(define v (vector 0 1 2 3 4))
(vector-copy! v 1 v 0 4)
v                                           ; => #(0 0 1 2 3)
(define p (list 1))
(define a (vector p 2))
(define b (vector-copy a))
(vector-set! b 1 9)
(set-car! p 7)
(list a b)                                  ; => (#((7) 2) #((7) 9))
(guard (e (else #t)) (vector-fill! '#() 1) #f) ; => #t
```

## ListとStringの変換

| 手続き | 引数数・型 | 返り値と共有 |
|---|---|---|
| `(vector->list v [start [end]])` | 1〜3、Vectorと境界 | 要素を浅く保持する新しい可変List |
| `(list->vector list)` | 1、proper List | 要素を浅く保持する新しい可変Vector |
| `(vector->string v [start [end]])` | 1〜3、Vectorと境界 | 範囲内のCharacterから作る新しい可変String |
| `(string->vector s [start [end]])` | 1〜3、Stringと文字境界 | Characterを格納する新しい可変Vector |

ListのPair、Vectorの格納領域、Stringの文字領域を入力と共有しません。
VectorとListの変換では、格納した可変の要素自体は浅く共有します。
list->vectorはimproper Listと循環Listでエラーです。
vector->stringは選択した範囲にCharacter以外があればエラーです。

```scheme
(vector->list '#(1 2 3) 1 3)                 ; => (2 3)
(list->vector '(1 2))                        ; => #(1 2)
(vector->string (string->vector "Aλ😀" 1 3)) ; => "λ😀"
```

## Bytevectorの手続き

| 手続き | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(bytevector? obj)` | 1、任意 | Bytevectorなら#t |
| `(make-bytevector k [byte])` | 1または2、kとbyte | kバイトの新しい可変Bytevector。byte省略時の内容は未規定 |
| `(bytevector byte...)` | 0以上、byte | 引数を格納する新しい可変Bytevector |
| `(bytevector-length bv)` | 1、Bytevector | バイト数の正確な整数 |
| `(bytevector-u8-ref bv k)` | 2、Bytevectorとk | 0〜255の正確な整数 |
| `(bytevector-u8-set! bv k byte)` | 3、Bytevector、k、byte | 元のバイトを変更。結果は未規定 |
| `(bytevector-copy bv [start [end]])` | 1〜3、Bytevectorとバイト境界 | 指定範囲をコピーした新しい可変Bytevector |
| `(bytevector-copy! to at from [start [end]])` | 3〜5、Bytevector、位置、Bytevectorとバイト境界 | 元のtoへコピー。結果は未規定 |
| `(bytevector-append bv...)` | 0以上、Bytevector | 連結した新しい可変Bytevector。空なら空Bytevector |

コピーと連結の結果は、入力とバイト領域を共有しません。
copy!の重複、事前の境界検証と不変性の規則はvector-copy!と同じです。
bytevector-u8-refの添字は長さより小さい必要があります。
make-bytevectorの長さが0でも、不正なbyte引数はエラーです。

```scheme
(bytevector 0 127 255)                ; => #u8(0 127 255)
(make-bytevector 3 255)               ; => #u8(255 255 255)
(bytevector-copy #u8(1 2 3) 1 3)      ; => #u8(2 3)
(bytevector-append #u8(1 2) #u8(3))    ; => #u8(1 2 3)
```

## UTF-8との変換

| 手続き | 引数数・型 | 返り値 |
|---|---|---|
| `(utf8->string bv [start [end]])` | 1〜3、Bytevectorとバイト境界 | UTF-8を復号した新しい可変String |
| `(string->utf8 s [start [end]])` | 1〜3、Stringと文字境界 | UTF-8に符号化した新しい可変Bytevector |

選択した範囲だけを変換し、入力の可変領域は共有しません。
Bytevector側はバイト単位、String側はコードポイント単位です。
UTF-8の継続バイトの途中など、不正な範囲の内容を復号するとエラーです。
不正な内容を置換文字へ暗黙に置き換えません。
NUL、補助平面の文字、空の範囲も扱います。

```scheme
(string->utf8 "Aλ😀" 1 2)                 ; => #u8(206 187)
(utf8->string #u8(65 206 187 66) 1 3)      ; => "λ"
(utf8->string #u8(255 65 255) 1 2)         ; => "A"
(guard (e (else #t)) (utf8->string #u8(206)) #f) ; => #t
```

## vector-mapとvector-for-each

| 手続き | 引数数・型 | 返り値と副作用 |
|---|---|---|
| `(vector-map proc v v...)` | 2以上、手続きと1個以上のVector | 対応する要素へprocを適用した単一値を格納する新しい可変Vector |
| `(vector-for-each proc v v...)` | 2以上、同じ型 | 先頭から順にprocを呼ぶ。結果は未規定 |

最も短いVectorの長さで停止します。
procの引数数はVectorの個数と同じです。
vector-mapのprocが複数値や値なしで返るとエラーで、vector-for-eachでは結果を捨てます。
mapの適用順は規格上未規定ですが、この実装では先頭から呼びます。
コールバックの副作用は発生し、同じ動的環境で実行します。
結果は独立したVectorですが、procが返した可変オブジェクトは浅く保持します。

VMのInvoke/Resume契約を使い、保存した継続を複数回再開できます。
以前返したVectorの格納領域は変更しません。
空の入力ではprocを呼びません。

```scheme
(vector-map + '#(1 2 3) '#(4 5))             ; => #(5 7)
(define total 0)
(vector-for-each (lambda (a b) (set! total (+ total a b))) '#(1 2 3) '#(4 5))
total                                        ; => 12
```

## 検証と残る接続

`internal/core/vectors_test.go`で使用例、全27名の登録、共有と独立コピー、境界、UTF-8、継続と2万要素の反復を検証します。
`internal/scheme`でbulk操作の重複、変更前の範囲検証、空の不変オブジェクトへの変更を確認します。
#67で旧Guri機能を除去して公開CLIへ接続し、公開側の17件と62件も成功しました。内部実装の単体検証と別に測定しています。
