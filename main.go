package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/downloader"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/texconfig"
	"github.com/e-chan1007/hayatex/internal/utils"
)

func main() {
	config := &config.Config{
		MirrorURL:       "https://mirror.ctan.org/systems/texlive/tlnet/",
		TexDir:          "./texlive_test",
		Arch:            "x86_64-linux",
		InstallDocFiles: false,
		InstallSrcFiles: false,
	}
	config.TexDir, _ = filepath.Abs(config.TexDir)

	if os := runtime.GOOS; os == "windows" {
		config.Arch = "windows"
	}

	roots := []string{
		"collection-basic",
		"collection-langjapanese",
		"collection-latexextra",
		"collection-mathscience",
		"collection-binextra",
	}

	fmt.Println("⏳ Parsing texlive.tlpdb...")
	tlpdb, err := resolver.RetrieveTLDatabase(config.MirrorURL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("🔍 Resolving dependencies...")
	deps := resolver.ResolveDependencies(roots, tlpdb, config.Arch)
	if config.Arch == "windows" {
		(*deps)["tlperl.windows"] = (*tlpdb)["tlperl.windows"]
	}

	start := time.Now()
	dl := downloader.New(config)
	downloadEstimate := dl.EstimateDownload(deps)
	progressChan := make(chan *downloader.InstallProgress)

	fmt.Printf("📦 Found %d files to download (Total Size: %s)\n", downloadEstimate.TotalDownloads, utils.FormatBytes(downloadEstimate.TotalDownloadSize, "B"))

	go func() {
		for progress := range progressChan {
			fmt.Printf("\r📥 Downloaded: %d / %d (%s | Extracted %s)", progress.CompletedCount, downloadEstimate.TotalDownloads, utils.FormatBytes(progress.DownloadedSize, "B"), utils.FormatBytes(progress.ExtractedSize, "B"))
		}
	}()

	fmt.Println("🚀 Starting parallel installation...")
	ctx := context.Background()
	if err := dl.InstallPackages(ctx, deps, progressChan); err != nil {
		log.Fatal(err)
	}

	if err := texconfig.SaveLocalTLPDB(config, tlpdb, deps); err != nil {
		log.Printf("Failed to write texlive.tlpdb: %v", err)
	}
	fmt.Printf("\n✅ Done! Installation took %v\n", time.Since(start))

	fmtutilConfigPath, err := texconfig.GenerateFmtutilConfig(config.TexDir, deps)

	if err != nil {
		log.Printf("Failed to generate fmtutil.cnf: %v", err)
	}

	texconfig.GenerateUpdmapConfig(config.TexDir, deps)

	binDir := filepath.Join(config.TexDir, "bin", config.Arch)

	env := os.Environ()
	newPath := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	if config.Arch == "windows" {
		perlDir := filepath.Join(config.TexDir, "tlpkg", "tlperl", "bin")
		newPath = perlDir + string(os.PathListSeparator) + newPath
	}
	env = utils.SetEnv(env, "PATH", newPath)
	env = utils.SetEnv(env, "TEXMFROOT", config.TexDir)
	env = utils.SetEnv(env, "PERL5LIB", filepath.Join(config.TexDir, "tlpkg"))

	logFile, err := os.Create(filepath.Join(config.TexDir, "install.log"))
	if err != nil {
		log.Printf("Failed to create log file: %v", err)
	}
	defer logFile.Close()
	out := io.MultiWriter(logFile)

	newCommand := func(name string, args ...string) *exec.Cmd {
		cmdPath, err := utils.ResolveExecutable(binDir, name)
		if err != nil {
			log.Fatalf("Failed to resolve executable for %s: %v", name, err)
		}
		cmd := exec.Command(cmdPath, args...)
		cmd.Dir = config.TexDir
		cmd.Env = env
		cmd.Stdout = out
		cmd.Stderr = out
		return cmd
	}

	fmt.Println("🛠️ Running tlmgr path add...")
	execCmd := newCommand("tlmgr", "path", "add")
	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ tlmgr path add failed: %v", err)
	}

	fmt.Println("🛠️ Running tlmgr path add...")
	execCmd = newCommand("tlmgr", "generate", "language")

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ tlmgr path add failed: %v", err)
	}

	texconfig.GenerateLsR(config.TexDir)

	fmt.Println("🛠️ Running fmtutil-sys --all...")
	execCmd = newCommand("fmtutil-sys", "--all", "--cnffile", fmtutilConfigPath, "--nohash")

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ fmtutil-sys failed: %v", err)
	}

	fmt.Println("🛠️ Running updmap-sys --syncwithtrees...")
	execCmd = newCommand("updmap-sys", "--quiet", "--syncwithtrees", "--force", "--nohash")
	execCmd.Stdin = strings.NewReader("y\n")

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ updmap-sys failed: %v", err)
	}

	fmt.Println("🛠️ Running updmap-sys...")
	execCmd = newCommand("updmap-sys", "--nohash")

	if err := execCmd.Run(); err != nil {
		log.Printf("⚠️ updmap-sys failed: %v", err)
	}

	texconfig.GenerateLsR(config.TexDir)
}
