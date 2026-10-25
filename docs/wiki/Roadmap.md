# バージョン計画

## 0.0.3：Schemeの公開CLI

- 標準Reader、コンパイラ、バイトコードVMをCLIへ接続しました。
- ファイル実行、標準入力のプログラム実行、REPL、`-I`、標準入出力、終了規則を提供します。
- 16標準ライブラリの557公開束縛をimportできます。
- 旧Guriの独自構文・型・組み込みの意味とルートGo APIを削除しました。
- サンプルとWikiをSchemeへ移行しました。
- REPLのbracketed pasteでは、貼り付け後のEnterまで入力をまとめ、途中のプロンプトを抑制します。
- 公開CLIの17件とChibi由来62件の回帰ケースが成功しました。

0.0.3はユーザー指定のバージョン番号です。
タグやGitHub Releaseの公開、R7RS-small準拠宣言をこの実装だけで確定したものではありません。

## 残る確認

- #95：非LinuxのポートReadyと実機検証。
- `r7rs` featureの広告、初期リリースの対象OSと許容する制限、準拠宣言の範囲の判断。
- Chibi原本の監査で保留した1判定と、任意精度指数表記の未採用を明示した受入確認。

[監査結果](https://github.com/0x6d61/guri/wiki/R7RS-Audit)と[設計判断](https://github.com/0x6d61/guri/wiki/R7RS-Conformance)を参照してください。
独自拡張や組み込み用APIは、標準への移行と受入を終えた後の別段階で検討します。

## 過去の仕様

0.0.1のAST評価器、Map/Keyword、Go Slice互換Vector、String/Bytes、独自構文と、0.0.2の末尾呼び出し・apply実装はGit履歴に保存しています。
現在のリファレンスでは旧機能を利用可能として説明しません。
旧テスト56件の標準での代替・廃止理由は`docs/r7rs/legacy-tests.csv`で追跡します。
