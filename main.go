package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/ui"
	"github.com/e-chan1007/hayatex/internal/utils"
)

func main() {
	profile := flag.String("profile", "", "Path to configuration profile")
	flag.Parse()

	cfg := (*config.Config)(nil)
	if *profile != "" {
		var err error
		cfg, err = config.LoadProfile(*profile)
		if err != nil {
			log.Fatalf("Failed to load profile: %v", err)
		}
	} else {
		cfg = config.NewDefaultConfig()
	}
	if cfg.Arch == "" {
		cfg.Arch = utils.DetectTeXLiveArch()
	}

	fmt.Println("Retrieving TeX Live package database...")

	var err error
	cfg.MirrorURL, err = utils.ResolveMirror(cfg.MirrorURL)
	if err != nil {
		log.Fatalf("Failed to resolve mirror URL: %v", err)
	}

	tlpdb, err := resolver.RetrieveTLDatabase(cfg.MirrorURL)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.TexDir == "" {
		cfg.SetDefaultTeXDir(tlpdb.Year)
	}

	m := ui.NewRootModel(cfg, tlpdb, *profile != "")
	p := tea.NewProgram(m)
	m.SetProgram(p)

	fmt.Print("\033[1A\033[K") // remove last line

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v", err)
		os.Exit(1)
	}

	// install(config)

}
