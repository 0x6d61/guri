# Promiseと遅延評価

scheme lazyの5束縛を内部Runtimeのimportへ接続しました。
公開CLIと公開Go APIの切替は#67で扱います。
基準は[R7RS-small §4.2.5](https://standards.scheme.org/corrected-r7rs/r7rs-Z-H-6.html#TAG:__tex2page_sec_4.2.5)です。

## 構文と手続き

| 構文 | 引数数・型 | 返り値と変更 |
|---|---|---|
| `(delay expression)` | 式1個。作成時には評価しない | 式とレキシカル環境を保持する新しいPromise |
| `(delay-force expression)` | 式1個。force時の結果はPromise | forceの連鎖を末尾で進める新しいPromise |
| `(force promise)` | 1、Promise | 未計算なら計算し、結果をメモ化して返す。計算済みなら同じ値を返す |
| `(promise? obj)` | 1、任意 | Promiseなら#t。それ以外は#f。計算を開始しない |
| `(make-promise obj)` | 1、任意 | 計算済みPromiseを返す。objがPromiseなら同じobjを返す |

これらはscheme baseには公開せず、scheme lazyからimportします。
Promiseは任意の値を保持でき、手続きや可変Vectorを値として保持することもできます。
Promiseを手続きとして呼び出す機能や、算術などで暗黙にforceする機能は追加しません。
forceに非Promiseを渡すと通常のerror-objectを送出します。
R7RSが許す「非Promiseをそのまま返す」任意拡張は採用しません。

```scheme
(import (scheme base) (scheme lazy))
(define count 0)
(define p (delay (begin (set! count (+ count 1)) 42)))
(define before count)
(define first (force p))
(define second (force p))
(list before first second count) ; => (0 42 42 1)
```

## メモ化と共有

delayの式はPromiseを作った時のレキシカルな束縛を参照します。
作成後にset!で変更した束縛も、force時には変更後の値を参照します。
正常復帰して最初に確定した値を保存し、後から同じPromiseをforceしても式を評価し直しません。
変数への代入や引数への受け渡しでPromiseをコピーせず、同じメモ化状態を共有します。
値がVectorなどの可変オブジェクトなら同じオブジェクトを返し、内容を複製しません。

```scheme
(import (scheme base) (scheme lazy))
(define p (delay (vector 1)))
(define alias p)
(vector-set! (force p) 0 9)
(vector-ref (force alias) 0) ; => 9
```

```scheme
(import (scheme base) (scheme lazy))
(define x 1)
(define p (delay x))
(set! x 42)
(force p) ; => 42
```

make-promiseの引数は通常の手続きの引数として先に評価します。
その値が既存のPromiseなら、追加のラッパーを作りません。
一方、delayの式がPromiseを値として返す場合は、その内側のPromiseを値として保存します。
forceで自動的に内側まで計算することはありません。

```scheme
(import (scheme base) (scheme lazy))
(define p (delay 42))
(list (eq? p (make-promise p))
      (promise? (force (delay p)))
      (force (force (delay p)))) ; => (#t #t 42)
```

## delay-forceと空間

delay-forceの式が返したPromiseとメモ化状態を共有し、その先をVMの次の呼び出しとして計算します。
通常のdelayでforceを入れ子にした場合と異なり、各段の返り値を待つ不要な継続を積み続けません。
参照されなくなった古い計算も保持しないよう、共有状態への転送と計算用クロージャの解放を行います。
[SRFI 45の遅延評価の説明](https://srfi.schemers.org/srfi-45/srfi-45.html)も参考にし、実装は既存VMのInvoke/Resumeと非公開Recordで構成しています。

```scheme
(import (scheme base) (scheme lazy))
(define (skip n)
  (delay-force
    (if (= n 0)
        (delay 42)
        (skip (- n 1)))))
(force (skip 100000)) ; => 42
```

10万段の連鎖で、VMフレーム数とGC後の保持メモリの上限をテストしています。
利用者がストリームの過去の要素や大量の継続を保持した場合まで、保持メモリを一定にするものではありません。
Promiseが自分自身を未完の結果として返し続ける場合は停止しないことがあります。
この場合もVMのGo contextによるキャンセルを確認します。

## 再入と継続

遅延した式の評価中に、同じPromiseを再びforceすることもできます。
再入によって内側の計算が先に値を確定した場合、外側が後から返した値で上書きしません。
同じPromiseへ転送済みの別名も、この確定した状態を共有します。

```scheme
(import (scheme base) (scheme lazy))
(define count 0)
(define p
  (delay
    (begin
      (set! count (+ count 1))
      (if (= count 1)
          (begin (force p) 999)
          42))))
(define first (force p))
(define second (force p))
(list first second count) ; => (42 42 2)
```

call/ccで計算の途中から脱出した場合、正常復帰していない値をメモ化しません。
捕捉した継続へ明示的に再入して正常復帰すると、その値を確定できます。
確定後に古い継続へ再入しても、既に保存した値を置き換えません。
明示的な継続再入で利用者の式を再実行することと、計算済みPromiseへの通常のforceは区別します。

## 動的パラメータと例外

動的パラメータと例外ハンドラは、Promiseの作成時に固定しません。
未完の計算をforceした呼び出しの動的環境を使用します。
その値をメモ化した後は、別のパラメータ値でforceしても同じ値を返します。

```scheme
(import (scheme base) (scheme lazy))
(define setting (make-parameter 1))
(define p (delay (setting)))
(define result (parameterize ((setting 42)) (force p)))
(list result (parameterize ((setting 99)) (force p))) ; => (42 42)
```

例外で計算から脱出した場合、例外を正常な返り値として保存しません。
そのPromiseを後からforceした場合は、未完の計算を再び実行できます。
副作用や既に完了した内側の計算を巻き戻すことはありません。
raise-continuableに対するハンドラの値などを使って計算が正常復帰した場合は、その結果を保存します。

```scheme
(import (scheme base) (scheme lazy))
(define count 0)
(define p
  (delay
    (begin
      (set! count (+ count 1))
      (if (= count 1) (error "retry") 42))))
(guard (e (else #t)) (force p))
(define first (force p))
(define second (force p))
(list first second count) ; => (42 42 2)
```

## エラーと未規定の動作

非Promiseへのforceや、delay-forceの式がPromiseを返さない場合はerror-objectを送出します。
delayとdelay-forceの不正な構文は位置付きSyntaxErrorです。
手続きの引数数の不一致は、既存VMと同じく自動raiseへ未接続です。

R7RSでは、delayの式が多値を返す効果は未規定です。
この実装は未完の結果を確定する際に単一値を要求し、値が0個または複数個の場合はerror-objectを送出します。
これを、全てのR7RS処理系に必要なエラー動作とは扱いません。
Promiseの表示は内部の不透明な表記であり、Readerによる復元は提供しません。
