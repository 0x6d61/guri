# Guriの言語仕様と関数リファレンス

Guri 0.0.3は、Goで実装したバイトコードVMのScheme処理系です。
単体CLIでのアプリケーション開発・実行を主用途とします。
16標準ライブラリを公開CLIからimportできます。
R7RS-smallの準拠宣言は保留しており、未確認事項を[準拠計画](https://github.com/0x6d61/guri/wiki/R7RS-Conformance)と[監査結果](https://github.com/0x6d61/guri/wiki/R7RS-Audit)で追跡します。

| ページ | 内容 |
|---|---|
| [CLIとインストール](https://github.com/0x6d61/guri/wiki/CLI) | ビルド、REPL、貼り付け、ファイル実行、検索先、終了コード |
| [言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification) | 型、Reader、評価、可変性とエラー |
| [構文リファレンス](https://github.com/0x6d61/guri/wiki/Special-Forms) | コア構文、マクロ、派生構文と使用例 |
| [関数・公開束縛一覧](https://github.com/0x6d61/guri/wiki/Functions) | 全16ライブラリ、557公開束縛と分野別リファレンス |
| [ライブラリ](https://github.com/0x6d61/guri/wiki/Libraries) | import、define-library、配置と初期化 |
| [環境、eval、loadとREPL](https://github.com/0x6d61/guri/wiki/Evaluation-REPL) | 評価環境と対話実行 |
| [バイトコードVM](https://github.com/0x6d61/guri/wiki/Bytecode-VM) | 命令、継続とproper tail recursion |
| [適合性テスト](https://github.com/0x6d61/guri/wiki/R7RS-Testing) | テストの実行方法と結果の分類 |
| [バージョン計画](https://github.com/0x6d61/guri/wiki/Roadmap) | 移行完了範囲と残る作業 |

プログラムは必要なライブラリをimportしてから実行します。

```scheme
(import (scheme base) (scheme write))
(display (+ 20 22))
(newline) ; 42
```

旧Guriの独自構文・型とルートGo APIは0.0.3で削除しました。
過去の仕様はGitの履歴に残し、現在のリファレンスと区別します。
