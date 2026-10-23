# Guriの言語仕様と関数リファレンス

このWikiは、Guriで使用できる構文と関数を説明します。
v0.0.1の基本機能である`defn`、String／Bytes、while、cond、errorを含みます。
v0.0.2向けの末尾呼び出し対応とapplyを追加しています。
R7RS-smallへの移行を計画しています。
現在の処理系はR7RS-small準拠ではありません。
[準拠計画と要求追跡](https://github.com/0x6d61/guri/wiki/R7RS-Conformance)では、目標仕様と設計の未決事項を現在の仕様から区別します。
実装済みの機能と残りの計画は [バージョン計画](https://github.com/0x6d61/guri/wiki/Roadmap) で区別します。

| ページ | 内容 |
|---|---|
| [言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification) | 型、Reader、評価規則、共有、エラー |
| [特殊形式](https://github.com/0x6d61/guri/wiki/Special-Forms) | `def`、`defn`、`fn`、条件分岐、束縛など全12形式 |
| [関数リファレンス](https://github.com/0x6d61/guri/wiki/Functions) | 全30関数の引数、型、返り値、変更の有無、使用例、エラー |
| [準拠計画と要求追跡](https://github.com/0x6d61/guri/wiki/R7RS-Conformance) | R7RS-smallの要求台帳、差分、設計相談と受入条件 |
| [標準Reader](https://github.com/0x6d61/guri/wiki/R7RS-Reader) | R7RS datumの読み取りと、実行への未接続範囲 |
| [内部データモデル](https://github.com/0x6d61/guri/wiki/Runtime-Data-Model) | R7RS値層の契約、unitテストと未接続の機能 |
| [適合性テスト](https://github.com/0x6d61/guri/wiki/R7RS-Testing) | ケースの実行方法、結果の分類と未対応の測定 |
| [バージョン計画](https://github.com/0x6d61/guri/wiki/Roadmap) | R7RS-smallへの移行順序と既存バージョンの実装記録 |

## 実行方法

Go 1.26以降で、リポジトリのルートから次のコマンドを実行します。

```sh
go run ./cmd/guri examples/counter.guri
```

CLIは各式を評価します。
値を画面に表示するときは `println` を使用します。
各ページのコード例では、`; =>` が出力を示します。
コードブロックごとに独立した環境で実行します。

```clojure
(defn add [x y] (+ x y))
(println (add 2 3)) ; => 5
```

## 現在の対応範囲

- Stringの `len`、`nth`、`slice` はUnicodeコードポイント単位です。
- `b"..."` は不変のBytesで、バイト単位の `len`、`nth`、`slice` に対応します。
- 入出力の組み込み関数は `println` です。バイト入力と改行なしのバイト出力は未実装です。
- `while` と末尾呼び出しの反復で、回数に比例してGoのスタックを増やさず実行できます。
- `cond` で条件を順に調べ、最初に一致した分岐だけを実行できます。
- `(error message)` でUserErrorを発生させ、現在の評価を中断できます。
- `apply` でVectorまたはListを引数に展開できます。apply経由の末尾呼び出しも評価ループで処理します。

[括弧対応表を作る例](https://github.com/0x6d61/guri/blob/master/examples/search-brackets.guri) は、Stringのnth、while、condを組み合わせて動作します。
正常時は対応表のMapを返し、括弧不一致はerrorで中断します。

未実装の機能を使用可能な関数として記載せず、追加した機能の仕様と使用例を実装と同じPRで更新します。
原稿は [docs/wiki](https://github.com/0x6d61/guri/tree/master/docs/wiki) で管理します。

- [バイトコードVMの設計](https://github.com/0x6d61/guri/wiki/Bytecode-VM)
