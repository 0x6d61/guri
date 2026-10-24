- [ホーム](https://github.com/0x6d61/guri/wiki)
- [言語仕様](https://github.com/0x6d61/guri/wiki/Language-Specification)
- [特殊形式](https://github.com/0x6d61/guri/wiki/Special-Forms)
- [関数リファレンス](https://github.com/0x6d61/guri/wiki/Functions)
- [バージョン計画](https://github.com/0x6d61/guri/wiki/Roadmap)
- [R7RS-small準拠計画](https://github.com/0x6d61/guri/wiki/R7RS-Conformance)
- [適合性テスト](https://github.com/0x6d61/guri/wiki/R7RS-Testing)
- [内部データモデル](https://github.com/0x6d61/guri/wiki/Runtime-Data-Model)
- [標準Reader](https://github.com/0x6d61/guri/wiki/R7RS-Reader)
- [バイトコードVMの設計](https://github.com/0x6d61/guri/wiki/Bytecode-VM)
- [Schemeのコア構文](https://github.com/0x6d61/guri/wiki/Core-Syntax)
- [syntax-rulesマクロ](https://github.com/0x6d61/guri/wiki/Syntax-Rules)

- [派生構文と準引用](https://github.com/0x6d61/guri/wiki/Derived-Syntax)

- [継続と複数値の制御機能](https://github.com/0x6d61/guri/wiki/Control-Features)

- [dynamic-windと動的な実行範囲](https://github.com/0x6d61/guri/wiki/Dynamic-Wind)

- [例外と動的パラメータ](https://github.com/0x6d61/guri/wiki/Exceptions-Parameters)

- [標準数値手続き](https://github.com/0x6d61/guri/wiki/Numbers): 内部VMの数値塔、比較、整数演算、数学関数、数値と文字列の変換。公開CLIは未接続で、importは内部Runtimeへ接続済み。

- [Pair、List、Symbolと等価性](https://github.com/0x6d61/guri/wiki/Pairs-Lists-Symbols): 内部VMの63手続きと、循環の比較、共有とコピー、mapと探索の継続。

- [CharacterとString](https://github.com/0x6d61/guri/wiki/Characters-Strings): 内部VMの50手続き、Unicodeの分類と大小変換、可変Stringと継続。

- [VectorとBytevector](https://github.com/0x6d61/guri/wiki/Vectors-Bytevectors): 内部VMの27手続き、固定長と浅いコピー、重複copy、UTF-8とコールバック継続。

- [datum labelとグラフの読み書き](https://github.com/0x6d61/guri/wiki/Graph-IO): 内部Reader/Writerの共有・循環と3出力方式。標準read/writeのポート束縛も内部VMへ接続済み。

- [Record型定義](https://github.com/0x6d61/guri/wiki/Records): 内部VMのdefine-record-typeと、生成するコンストラクタ・述語・取得／変更手続き。

- [テキスト・バイナリポート](https://github.com/0x6d61/guri/wiki/Ports): 内部VMの38束縛、開閉と借用、文字／バイトの入出力、current portsと継続。datumのread/write、ファイルと公開CLIは後続。

- [datumとファイルの入出力](https://github.com/0x6d61/guri/wiki/Datum-File-IO): 内部VMの13手続き、ストリームReader、write/display、ファイルの所有と継続。公開CLIは後続。importは内部Runtimeへ接続済み。

- [ライブラリとプログラムの環境](https://github.com/0x6d61/guri/wiki/Libraries): define-library/import、インストール先と追加検索パスの規則、10標準ライブラリと未提供の6ライブラリ、初期化と継続。

- [Promiseと遅延評価](https://github.com/0x6d61/guri/wiki/Lazy-Evaluation): scheme lazyの5束縛、メモ化と再入、force時の動的環境、delay-forceの空間要件。
