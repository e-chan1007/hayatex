package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/downloader"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/texconfig"
	"github.com/e-chan1007/hayatex/internal/utils"
)

func main() {
	config := &config.Config{
		MirrorURL:          "https://mirror.ctan.org/systems/texlive/tlnet/",
		TexDir:             "./texlive_test",
		Arch:               utils.DetectTeXLiveArch(),
		AddPath:            true,
		InstallDocFiles:    false,
		InstallSrcFiles:    false,
		InstallForAllUsers: false,
		SysBinDir:          "/usr/local/bin",
		SysManDir:          "/usr/local/share/man",
		SysInfoDir:         "/usr/local/share/info",
	}

	if config.Arch == "" {
		log.Fatal("Unsupported architecture")
	}

	config.TexDir, _ = filepath.Abs(config.TexDir)

	var err error

	err = utils.CheckWritePermission(config.TexDir)
	if err != nil {
		log.Fatalf("No write permission for the installation directory: %v", err)
	}

	if config.MirrorURL, err = utils.ResolveMirror(config.MirrorURL); err != nil {
		log.Fatalf("Failed to resolve mirror URL: %v", err)
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

	logFile, err := os.Create(filepath.Join(config.TexDir, "install.log"))
	if err != nil {
		log.Printf("Failed to create log file: %v", err)
	}
	defer logFile.Close()

	texCommandExecutor := utils.TeXCommandExecutor(config, logFile)

	if config.AddPath {
		start = time.Now()
		fmt.Println("🛠️ Adding Path ...")
		if err := texconfig.AddSystemLinks(config); err != nil {
			log.Printf("⚠️ Adding Path failed: %v", err)
		}
		fmt.Printf("✅ Adding Path completed in %v\n", time.Since(start))
	}

	start = time.Now()
	fmt.Println("🛠️ Running generate language(rewritten)...")
	if err := texconfig.GenerateLanguageConfig(config.TexDir, deps); err != nil {
		log.Printf("⚠️ Failed to generate language config: %v", err)
	}
	fmt.Printf("✅ Generated language config in %v\n", time.Since(start))

	start = time.Now()
	texconfig.GenerateLsR(config.TexDir)
	fmt.Printf("✅ Generated ls-R in %v\n", time.Since(start))

	start = time.Now()
	fmt.Println("🛠️ Running fmtutil(rewritten)...")
	texconfig.ExecuteFormatCommands(ctx, config, deps, &texCommandExecutor)
	fmt.Printf("✅ fmtutil(rewritten) completed in %v\n", time.Since(start))

	start = time.Now()
	fmt.Println("🛠️ Running updmap(rewritten)...")
	if err := texconfig.ExecuteUpdmap(config.TexDir, deps); err != nil {
		log.Printf("⚠️ updmap-sys failed: %v", err)
	}
	fmt.Printf("✅ updmap-sys completed in %v\n", time.Since(start))

	texconfig.GenerateLsR(config.TexDir)
}
