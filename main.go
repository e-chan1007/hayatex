package main

import (
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

	fmt.Println("Retrieving TeX Live package database...")

	tlpdb, err := resolver.RetrieveTLDatabase(config.MirrorURL)
	if err != nil {
		log.Fatal(err)
	}

	m := ui.NewRootModel(config, tlpdb)
	p := tea.NewProgram(m)
	m.SetProgram(p)

	fmt.Print("\033[1A\033[K") // remove last line

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v", err)
		os.Exit(1)
	}

	// install(config)

}
