# HayaTeX: A Fast TeX Live Installer

HayaTeX は、Go言語で書かれたTUIベースの高速なTeX Liveインストーラーです。
並列ダウンロードと構成の最適化により、TeX Live環境のセットアップにかかる時間を大幅に短縮します。

> [!TIP]
> HayaTeXは、日本語の「疾風(はやて)」と「TeX」にちなんで名付けられ、発音も「はやて」です。
> 新幹線の名前としても知られるように、速く効率的なTeX Liveのインストールの象徴としています。

## 特徴
- ダウンロード及び構成コマンドの並列実行による高速インストール
- TeX Liveのプロファイルを使用した自動構成

> [!WARNING]
> プロファイルを利用したインストールは現在部分的に利用可能ですが、`select_scheme`などの一部のプロファイルオプションのみをサポートしています。その他のオプションは無視されるか、予期しない動作を引き起こす可能性があります。プロファイルを利用したインストールで問題が発生した場合は、[構成の読み込みに関連するソースコード](internal/config/config.go)を参照してください。

## インストール
最新のHayaTeXリリースは、[GitHubのリリースページ](https://github.com/e-chan1007/hayatex/releases)からダウンロードできます。
または、Goを使用してソースからビルドすることもできます。

```bash
git clone https://github.com/e-chan1007/hayatex.git
cd hayatex
go build .
```

## 使い方
コンパイルされたバイナリを実行し、プロンプトに従ってください。
```bash
./hayatex
```

構成プロファイルやミラーリポジトリのURLを指定することもできます。

```bash
./hayatex --profile path/to/texlive.profile --repository http://example.com/ctan/
```

> [!WARNING]
> HayaTeXはデフォルトで、フォーマットコマンド(`fmtutil`)の高速な再実装を使用していますが、一部の環境では互換性に問題が発生する可能性があります。インストール後に問題が発生した場合は、`fmtutil-sys --all`を実行してフォーマットファイルを再生成してみてください。または、インストール時に互換モードを有効にして、標準の`fmtutil`コマンドを使用することもできます。これによりインストール時間が増加する可能性がありますが、特定のTeX Live構成との互換性が向上する可能性があります。

```bash
./hayatex --compat
```
