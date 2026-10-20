# Guriの言語仕様と関数リファレンス

このWikiは、Guriで使用できる構文と関数を説明します。
初期仕様v0.0.1と、その後に追加した `defn` を含む現在の実装を対象にします。

| ページ | 内容 |
|---|---|
| [言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification) | 型、Reader、評価規則、共有、エラー |
| [特殊形式](https://github.com/0x6d61/guri/wiki/Special-Forms) | `def`、`defn`、`fn`、条件分岐、束縛など全10形式 |
| [関数リファレンス](https://github.com/0x6d61/guri/wiki/Functions) | 全28関数の引数、型、返り値、変更の有無、使用例、エラー |

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

- `nth` と `slice` はVector専用です。文字列には使用できません。
- `len` はStringのUTF-8バイト長を返します。
- 入出力の組み込み関数は `println` です。バイト入力と改行なしのバイト出力は未実装です。
- 繰り返し専用の構文と末尾呼び出し最適化は未実装です。再帰呼び出しはGoのスタックを使用します。

未実装の機能を使用可能な関数として記載せず、追加した機能の仕様と使用例を実装と同じPRで更新します。
原稿は [docs/wiki](https://github.com/0x6d61/guri/tree/master/docs/wiki) で管理します。
