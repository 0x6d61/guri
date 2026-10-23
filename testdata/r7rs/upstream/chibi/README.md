# Chibi SchemeのR7RSテスト原本

このディレクトリには、Chibi Schemeの次の固定版から取得した原本を保存します。
ファイルの内容は変更していません。

- リポジトリ：https://github.com/ashinn/chibi-scheme
- リビジョン：`b2087f3a320a56f79ede5c546fd45fdde582c4ed`
- ライセンス：BSD 3-Clause。[COPYING](COPYING)を参照してください。
- 採用記録：[Issue #70](https://github.com/0x6d61/guri/issues/70)。ユーザーの取り込み指示によりC44-01を承認。

| 保存先 | 元のパス | SHA-256 |
|---|---|---|
| r7rs-tests.scm | tests/r7rs-tests.scm | `004d215a38f990bbf2bfaa8b4a395aa59ef98d155096f8e31cc361b7812f7d6a` |
| COPYING | COPYING | `86805fe7af1bd20d5f5050d86c61d6fe038452cb08c6106a08761e010d37da43` |

元の著作権表示、ライセンス条件、免責文とファイル中の寄稿者表示を保持しています。
Guri本体のMITライセンスをこの原本や派生テストのライセンスとして扱いません。
バイト列を保持するため、該当ファイルの改行変換と空白の修正を`.gitattributes`で無効にしています。
取得後のunitテストとCIは、この原本へネットワークなしでアクセスします。

## 実行する範囲

原本全体を保存していますが、原本全体をGuriで実行する設定ではありません。
原本は`(chibi test)`に依存し、Unicode全体、数値塔と任意ライブラリを前提にします。
`lib/chibi/test.scm`やその追加ライブラリ群は取り込んでいません。

[独立した派生ケース](../../chibi/README.md)は別ディレクトリで管理します。
原本を成功する形へ書き換えず、出典の行と規格を確認して必要な実行用コードを追加します。
更新するときはリビジョン、ハッシュ、ライセンス、行番号と派生ケースの差分をPRで確認します。
