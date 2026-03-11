package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/e-chan1007/hayatex/internal/downloader"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/utils"
)

func main() {
	baseURL := "https://mirror.ctan.org/systems/texlive/tlnet/"
	// baseURL := "https://ftp.jaist.ac.jp/pub/CTAN/systems/texlive/tlnet/"

	texdir := "./texlive_test"

	arch := "x86_64-linux"
	if os := runtime.GOOS; os == "windows" {
		arch = "windows"
	}

	roots := []string{
		"collection-basic",
		"collection-langjapanese",
		"collection-latexextra",
		"collection-mathscience",
		"collection-binextra",
	}

	fmt.Println("⏳ Parsing texlive.tlpdb...")
	tlpdb, err := resolver.RetrieveTLDatabase(baseURL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("🔍 Resolving dependencies...")
	deps := resolver.ResolveDependencies(roots, tlpdb, arch)

	var totalSize uint64
	for _, p := range deps {
		totalSize += p.Container.Size
	}
	fmt.Printf("📦 Found %d packages (Total Download: %s)\n", len(deps), utils.FormatBytes(totalSize, "B"))

	fmt.Println("🚀 Starting parallel installation...")
	start := time.Now()

	dl := downloader.New(baseURL, texdir)
	ctx := context.Background()

	if arch == "windows" {
		deps["tlperl.windows"] = tlpdb["tlperl.windows"]
	}
	deps["00texlive.config"] = tlpdb["00texlive.config"]
	deps["00texlive.installation"] = createTeXLiveInstallationConfig(baseURL, arch)

	if err := dl.InstallPackages(ctx, deps); err != nil {
		log.Fatal(err)
	}

	if err := saveLocalTLPDB(filepath.Join(texdir, "tlpkg/texlive.tlpdb"), deps); err != nil {
		log.Printf("Failed to write texlive.tlpdb: %v", err)
	}

	generateFmtutilConfig(texdir, deps)
	generateUpdmapConfig(texdir, deps)

	fmt.Printf("\n✅ Done! Installation took %v\n", time.Since(start))

	// main.go の後半部分の修正案

	// 絶対パスを取得しておく
	absTexDir, _ := filepath.Abs(texdir)
	binDir := filepath.Join(absTexDir, "bin", arch)

	// 2. 環境変数の構築
	env := os.Environ()
	// PATH の構築
	newPath := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	if arch == "windows" {
		// Windowsは tlperl も PATH に含める必要がある
		perlDir := filepath.Join(absTexDir, "tlpkg", "tlperl", "bin")
		newPath = perlDir + string(os.PathListSeparator) + newPath
	}
	env = utils.SetEnv(env, "PATH", newPath)
	env = utils.SetEnv(env, "TEXMFROOT", absTexDir)
	env = utils.SetEnv(env, "PERL5LIB", filepath.Join(absTexDir, "tlpkg"))

	// 3. tlmgr の実行
	// Chdir する場合は、実行ファイル名はフルパスで指定するのが確実
	tlmgrCmd := "tlmgr"
	if arch == "windows" {
		tlmgrCmd = filepath.Join(binDir, "tlmgr.bat")
	} else {
		tlmgrCmd = filepath.Join(binDir, "tlmgr")
	}

	fmt.Println("🛠️ Running tlmgr path add...")
	execCmd := exec.Command(tlmgrCmd, "path", "add")
	execCmd.Dir = absTexDir // 作業ディレクトリを明示
	execCmd.Env = env
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ tlmgr path add failed: %v", err)
	}

	fmt.Println("🛠️ Running tlmgr path add...")
	execCmd = exec.Command(tlmgrCmd, "generate", "language")
	execCmd.Dir = absTexDir // 作業ディレクトリを明示
	execCmd.Env = env
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ tlmgr path add failed: %v", err)
	}
	generateLsRs(absTexDir)

	fmtutilCmd := "fmtutil-sys"
	if arch == "windows" {
		fmtutilCmd = filepath.Join(binDir, "fmtutil-sys.exe")
	} else {
		fmtutilCmd = filepath.Join(binDir, "fmtutil-sys")
	}

	fmt.Println("🛠️ Running fmtutil-sys --all...")
	myCnf := filepath.Join(texdir, "texmf-config", "web2c", "fmtutil.cnf")
	execCmd = exec.Command(fmtutilCmd, "--all", "--cnffile", myCnf, "--nohash")
	execCmd.Dir = absTexDir
	execCmd.Env = env
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ fmtutil-sys failed: %v", err)
	}

	updmapCmd := "updmap-sys"
	if arch == "windows" {
		updmapCmd = filepath.Join(binDir, "updmap-sys.exe")
	} else {
		updmapCmd = filepath.Join(binDir, "updmap-sys")
	}

	fmt.Println("🛠️ Running updmap-sys --syncwithtrees...")
	execCmd = exec.Command(updmapCmd, "--quiet", "--syncwithtrees", "--force", "--nohash")
	execCmd.Dir = absTexDir
	execCmd.Env = env
	execCmd.Stdin = strings.NewReader("y\n")
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ updmap-sys failed: %v", err)
	}

	fmt.Println("🛠️ Running updmap-sys...")
	execCmd = exec.Command(updmapCmd, "--nohash")
	execCmd.Dir = absTexDir
	execCmd.Env = env
	execCmd.Stdin = strings.NewReader("y\n")
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ updmap-sys failed: %v", err)
	}

	generateLsRs(absTexDir)
}

func createTeXLiveInstallationConfig(mirrorURL string, arch string) *resolver.TLPackage {
	return &resolver.TLPackage{
		Name: "00texlive.installation",
		Container: &resolver.TLContainerInfo{
			Size:     0,
			Checksum: "",
		},
		Depends: []string{
			"opt_location:" + mirrorURL,
			"opt_install_docfiles:0",
			"opt_install_srcfiles:0",
			"opt_create_formats:1",
			"setting_available_architectures:" + arch,
		},
	}
}

func generateLsRs(texdir string) {
	trees := []string{"texmf-dist", "texmf-var", "texmf-config"}

	for _, tree := range trees {
		rootPath := filepath.Join(texdir, tree)
		if _, err := os.Stat(rootPath); err == nil {
			err := generateLsR(rootPath)
			if err != nil {
				log.Printf("❌ Failed to generate ls-R for %s: %v", tree, err)
			}
		}
	}
}

func generateLsR(root string) error {
	out, err := os.Create(filepath.Join(root, "ls-R"))
	if err != nil {
		return err
	}
	defer out.Close()
	fmt.Fprintln(out, "% ls-R -- filename database for kpathsea; do not change this line.")
	db := make(map[string][]string)

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "ls-R" {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		dirKey := "./" + filepath.ToSlash(rel)
		db[dirKey] = append(db[dirKey], d.Name())
		return nil
	})

	for dir, files := range db {
		fmt.Fprintf(out, "\n%s:\n", dir)
		for _, f := range files {
			fmt.Fprintln(out, f)
		}
	}
	return nil
}

func generateFmtutilConfig(texdir string, deps resolver.TLDatabase) error {
	fmtutilDir := filepath.Join(texdir, "texmf-config", "web2c")
	if err := os.MkdirAll(fmtutilDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(fmtutilDir, "fmtutil.cnf"))
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("# Generated by HayaTeX\n")

	for _, pkg := range deps {
		for _, execLine := range pkg.Executes {
			if strings.HasPrefix(strings.ToLower(execLine), "addformat ") {
				line := parseAddFormat(execLine)
				if line != "" {
					f.WriteString(line + "\n")
				}
			}
		}
	}
	return nil
}

func parseAddFormat(execLine string) string {
	// AddFormat 以降を抽出
	content := strings.TrimSpace(strings.TrimPrefix(execLine, "AddFormat"))

	re := regexp.MustCompile(`(\w+)=("[^"]*"|[^\s]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	params := make(map[string]string)
	for _, m := range matches {
		key := m[1]
		val := strings.Trim(m[2], "\"")
		params[key] = val
	}

	if params["mode"] == "disabled" {
		return ""
	}

	name := params["name"]
	engine := params["engine"]
	patterns := params["patterns"]
	if patterns == "" {
		patterns = "-"
	}
	options := params["options"]

	if name == "" || engine == "" {
		return ""
	}

	return fmt.Sprintf("%s %s %s %s", name, engine, patterns, options)
}

func generateUpdmapConfig(texdir string, deps resolver.TLDatabase) error {
	updmapDir := filepath.Join(texdir, "texmf-config", "web2c")
	if err := os.MkdirAll(updmapDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(updmapDir, "updmap.cfg"))
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("# Generated by HayaTeX\n")

	for _, pkg := range deps {
		for _, execLine := range pkg.Executes {
			trimmed := strings.TrimSpace(execLine)
			lower := strings.ToLower(trimmed)

			if strings.HasPrefix(lower, "addmap") {
				mapFile := strings.TrimSpace(trimmed[len("addMap"):])
				f.WriteString(fmt.Sprintf("Map %s\n", mapFile))
			} else if strings.HasPrefix(lower, "addkanjimap") {
				mapFile := strings.TrimSpace(trimmed[len("addKanjiMap"):])
				f.WriteString(fmt.Sprintf("KanjiMap %s\n", mapFile))
			}
		}
	}
	return nil
}

func saveLocalTLPDB(targetPath string, deps resolver.TLDatabase) error {
	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(deps.ToString())
	return nil
}
