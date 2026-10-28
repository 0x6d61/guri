# バージョン計画

## 0.0.3：R7RS-small準拠

- 標準Reader、コンパイラ、バイトコードVMをCLIへ接続しました。
- ファイル実行、標準入力のプログラム実行、REPL、`-I`、標準入出力、終了規則を提供します。
- 16標準ライブラリの557公開束縛をimportできます。
- 旧Guriの独自構文・型・組み込みの意味とルートGo APIを削除しました。
- サンプルとWikiをSchemeへ移行しました。
- REPLのbracketed pasteでは、貼り付け後のEnterまで入力をまとめ、途中のプロンプトを抑制します。
- 公開CLIの17件とChibi由来62件の回帰ケースが成功しました。

0.0.3はユーザー指定のバージョン番号です。
#99でユーザー承認を受け、READMEとWikiでR7RS-small準拠を宣言しました。
featuresとcond-expandはr7rsを提供します。
Chibi原本の実行した1,214判定はすべて成功し、宣言前の保留1件は解消しました。
任意の精度指定s/f/d/lは未採用として記録します。

## OS別検証

#95でLinux、macOS、WindowsのポートReadyとOS別CIを検証しました。
確認した環境と結果は[結合監査](https://github.com/0x6d61/guri/wiki/R7RS-Audit)に記録します。

## 残る確認

- タグやGitHub Releaseによる配布、初期リリースの対象OSの確定。

[監査結果](https://github.com/0x6d61/guri/wiki/R7RS-Audit)と[設計判断](https://github.com/0x6d61/guri/wiki/R7RS-Conformance)を参照してください。
独自拡張や組み込み用APIは、準拠した基盤への追加として別段階で検討します。

## アプリケーション向け拡張

R7RS-smallの標準の意味を維持し、別ライブラリとして追加します。

1. #101：ファイルとディレクトリの操作。`(guri filesystem)`の8手続きを追加しました。
2. #102：外部プロセス実行とパイプ接続。未実装。
3. #103：JSONの読み書き。未実装。
4. #104：ハッシュテーブル、文字列処理、正規表現。未実装。
5. #105：Scheme用テストライブラリ。未実装。
6. #106：HTTPクライアント。未実装。
7. #107：ソース位置と関数名を持つスタックトレース。未実装。

## 過去の仕様

0.0.1のAST評価器、Map/Keyword、Go Slice互換Vector、String/Bytes、独自構文と、0.0.2の末尾呼び出し・apply実装はGit履歴に保存しています。
現在のリファレンスでは旧機能を利用可能として説明しません。
旧テスト56件の標準での代替・廃止理由は`docs/r7rs/legacy-tests.csv`で追跡します。
