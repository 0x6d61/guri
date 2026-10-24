# Wikiの原稿管理と公開

言語仕様と関数リファレンスの原稿を、実装と同じPRでレビューできるように `docs/wiki/` で管理します。
GitHub Wikiは公開先として使用します。

| 原稿 | 公開ページ |
|---|---|
| [Home.md](wiki/Home.md) | [ホーム](https://github.com/0x6d61/guri/wiki) |
| [Language-Specification.md](wiki/Language-Specification.md) | [言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification) |
| [Special-Forms.md](wiki/Special-Forms.md) | [特殊形式](https://github.com/0x6d61/guri/wiki/Special-Forms) |
| [Functions.md](wiki/Functions.md) | [関数リファレンス](https://github.com/0x6d61/guri/wiki/Functions) |
| [Roadmap.md](wiki/Roadmap.md) | [バージョン計画](https://github.com/0x6d61/guri/wiki/Roadmap) |
| [R7RS-Conformance.md](wiki/R7RS-Conformance.md) | [R7RS-small準拠計画](https://github.com/0x6d61/guri/wiki/R7RS-Conformance) |
| [R7RS-Testing.md](wiki/R7RS-Testing.md) | [適合性テスト](https://github.com/0x6d61/guri/wiki/R7RS-Testing) |
| [Runtime-Data-Model.md](wiki/Runtime-Data-Model.md) | [内部データモデル](https://github.com/0x6d61/guri/wiki/Runtime-Data-Model) |
| [R7RS-Reader.md](wiki/R7RS-Reader.md) | [標準Reader](https://github.com/0x6d61/guri/wiki/R7RS-Reader) |
| [Bytecode-VM.md](wiki/Bytecode-VM.md) | [バイトコードVMの設計](https://github.com/0x6d61/guri/wiki/Bytecode-VM) |
| [Core-Syntax.md](wiki/Core-Syntax.md) | [Schemeのコア構文](https://github.com/0x6d61/guri/wiki/Core-Syntax) |
| [Syntax-Rules.md](wiki/Syntax-Rules.md) | [syntax-rulesマクロ](https://github.com/0x6d61/guri/wiki/Syntax-Rules) |
| [Derived-Syntax.md](wiki/Derived-Syntax.md) | [派生構文と準引用](https://github.com/0x6d61/guri/wiki/Derived-Syntax) |
| [Control-Features.md](wiki/Control-Features.md) | [継続と複数値の制御機能](https://github.com/0x6d61/guri/wiki/Control-Features) |
| [Dynamic-Wind.md](wiki/Dynamic-Wind.md) | [dynamic-windと動的な実行範囲](https://github.com/0x6d61/guri/wiki/Dynamic-Wind) |
| [Exceptions-Parameters.md](wiki/Exceptions-Parameters.md) | [例外と動的パラメータ](https://github.com/0x6d61/guri/wiki/Exceptions-Parameters) |
| [Numbers.md](wiki/Numbers.md) | [標準数値手続き](https://github.com/0x6d61/guri/wiki/Numbers) |
| [Pairs-Lists-Symbols.md](wiki/Pairs-Lists-Symbols.md) | [Pair、List、Symbolと等価性](https://github.com/0x6d61/guri/wiki/Pairs-Lists-Symbols) |
| [Characters-Strings.md](wiki/Characters-Strings.md) | [CharacterとString](https://github.com/0x6d61/guri/wiki/Characters-Strings) |
| [Vectors-Bytevectors.md](wiki/Vectors-Bytevectors.md) | [VectorとBytevector](https://github.com/0x6d61/guri/wiki/Vectors-Bytevectors) |
| [Graph-IO.md](wiki/Graph-IO.md) | [datum labelとグラフの読み書き](https://github.com/0x6d61/guri/wiki/Graph-IO) |
| [Records.md](wiki/Records.md) | [Record型定義](https://github.com/0x6d61/guri/wiki/Records) |
| [Ports.md](wiki/Ports.md) | [テキスト・バイナリポート](https://github.com/0x6d61/guri/wiki/Ports) |
| [Datum-File-IO.md](wiki/Datum-File-IO.md) | [datumとファイルの入出力](https://github.com/0x6d61/guri/wiki/Datum-File-IO) |
| [Libraries.md](wiki/Libraries.md) | [ライブラリとプログラムの環境](https://github.com/0x6d61/guri/wiki/Libraries) |
| [_Sidebar.md](wiki/_Sidebar.md) | サイドバー |

## 変更時の手順

1. 対応するIssueと作業ブランチを作成します。
2. 実装を変更する場合はTDDで進め、原稿と使用例も同じブランチで更新します。
3. 各関数の引数、型、返り値、変更の有無、共有やコピー、エラーを実装と照合します。
4. 使用例を独立したInterpreterで実行し、期待する結果になることを確認します。
5. PRのCI成功を確認し、merge commitを作成してマージします。
6. 最新のmasterの原稿をWikiへ公開します。

## Wikiへの公開

初回だけ、GitHubのWeb画面でHomeページを作成する必要があります。
この初期化後に、WikiのGitリポジトリをcloneして公開できます。
[GitHub公式のWiki編集手順](https://docs.github.com/en/communities/documenting-your-project-with-wikis/adding-or-editing-wiki-pages) を参照してください。

最新のmasterへ切り替えて更新した後、リポジトリのルートで次のコマンドを実行します。
Wikiへのpush権限を持つ認証が必要です。

```sh
wiki_dir=$(mktemp -d)
git clone https://github.com/0x6d61/guri.wiki.git "$wiki_dir"
cp docs/wiki/*.md "$wiki_dir/"
git -C "$wiki_dir" add Home.md Language-Specification.md Special-Forms.md Functions.md Roadmap.md R7RS-Conformance.md R7RS-Testing.md Runtime-Data-Model.md R7RS-Reader.md Bytecode-VM.md Core-Syntax.md Syntax-Rules.md Derived-Syntax.md Control-Features.md Dynamic-Wind.md Exceptions-Parameters.md Numbers.md Pairs-Lists-Symbols.md Characters-Strings.md Vectors-Bytevectors.md Graph-IO.md Records.md Ports.md Datum-File-IO.md Libraries.md _Sidebar.md
git -C "$wiki_dir" diff --cached --check
git -C "$wiki_dir" diff --cached
```

差分を確認してから、変更がある場合にコミットしてpushします。
WikiはPRを作成する本体リポジトリとは別のGitリポジトリで、公開ブランチはmasterです。

```sh
git -C "$wiki_dir" commit -m "docs: 言語仕様と関数リファレンスを更新"
git -C "$wiki_dir" push origin master
```

公開後は上表の各ページとサイドバーを確認します。
Wiki上で直接修正した内容がある場合は、原稿への取り込みと差分確認を済ませてから同期します。
同期時に、今回の原稿以外のWikiページを削除しません。
