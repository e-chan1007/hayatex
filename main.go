package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/mirror"
	"github.com/e-chan1007/hayatex/internal/ui"
	"github.com/e-chan1007/hayatex/internal/utils"
)

var (
	version = "dev"
)

func PrintVersion() {
	fmt.Printf("HayaTeX Installer version %s\n", version)
}

func main() {
	PrintVersion()

	profile := flag.String("profile", "", "Path to configuration profile")
	repository := flag.String("repository", mirror.DefaultRepositoryURL, "TeX Live repository URL")
	compatMode := flag.Bool("compat", false, "Enable compatibility mode: use original fmtutil instead of faster implementation")
	flag.Parse()

	autoInstall := false

	var cfg *config.Config
	var err error

	if *profile != "" {
		cfg, err = config.LoadProfile(*profile)
		if err != nil {
			log.Fatalf("Failed to load profile: %v", err)
		}
		if cfg.Arch == "" {
			cfg.Arch = utils.DetectTeXLiveArch()
		}
		autoInstall = true
	}

	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}

	if *compatMode {
		cfg.CompatMode = true
	}

	if *repository != "" {
		cfg.MirrorURL = strings.TrimSuffix(strings.TrimSuffix(*repository, "/"), "/systems/texlive/tlnet")
	}

	ui.StartInstallWizard(cfg, autoInstall)
}
