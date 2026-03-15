package main

import (
	"flag"
	"log"
	"strings"

	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/mirror"
	"github.com/e-chan1007/hayatex/internal/ui"
	"github.com/e-chan1007/hayatex/internal/utils"
)

func main() {
	profile := flag.String("profile", "", "Path to configuration profile")
	repository := flag.String("repository", mirror.DefaultRepositoryURL, "TeX Live repository URL")
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

	if *repository != "" {
		cfg.MirrorURL = strings.TrimSuffix(strings.TrimSuffix(*repository, "/"), "/systems/texlive/tlnet")
	}

	ui.StartInstallWizard(cfg, autoInstall)
}
