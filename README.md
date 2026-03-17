# HayaTeX: A Fast TeX Live Installer

[日本語版はこちら (Japanese Version)](README_ja.md)

HayaTeX (pronounced "hayate") is a TUI-based fast TeX Live installer written in Go.

By leveraging parallel downloads and optimized configuration steps, HayaTeX significantly reduces the time required to set up a TeX Live environment.

> [!TIP]
> HayaTeX is named after the Japanese word "Hayate", which means "gale", a wind that blows hard and fast.
> The name reflects the goal of this installer: to provide a fast and efficient way to install TeX Live.
>
> The name is also known as the name of Japanese Shinkansen(bullet train).

## Features
- Fast installation using parallel downloads and execution of configuration commands
- Automatic configuration with TeX Live profiles

> [!WARNING]
> Profile-based installation is currently not fully supported.
> Only a limited set of profile options like `select_scheme` are supported, and some options may be ignored or cause unexpected behavior. If you encounter issues with profile-based installation, please refer to the [configuration code](internal/config/config.go) for the currently supported options.

## Installation
You can download the latest release of HayaTeX from the [GitHub releases page](https://github.com/e-chan1007/hayatex/releases).
Alternatively, you can build HayaTeX from source using Go:

```bash
git clone https://github.com/e-chan1007/hayatex.git
cd hayatex
go build .
```

## Usage
Just run the compiled binary and follow the prompts.
```bash
./hayatex
```

You can also specify a configuration profile and mirror repository URL:

```bash
./hayatex --profile path/to/texlive.profile --repository http://example.com/ctan/
```

> [!WARNING]
> By default, HayaTeX uses a faster implementation for executing format commands, which may cause issues in some environments. If you encounter problems after installation, try running `fmtutil-sys --all` to regenerate format files. Alternatively, in the installation step, you can enable compatibility mode to use the standard `fmtutil` command instead of the faster implementation. This may increase installation time but can improve compatibility with certain TeX Live configurations.
>
> ```bash
> ./hayatex -compat
>```
