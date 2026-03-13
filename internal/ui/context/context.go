package context

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/resolver"
)

type page int

const (
	SelectPage page = iota
	InstallPage
)

type RootContext struct {
	ActivePage     page
	Config         *config.Config
	Program        *tea.Program
	Style          *lipgloss.Style
	TerminalWidth  int
	TerminalHeight int
}

var rootContext *RootContext

func NewRootContext(config *config.Config, db *resolver.TLDatabase) RootContext {
	style := lipgloss.NewStyle().Padding(1, 2).Foreground(lipgloss.Color("#009977"))
	rootContext = &RootContext{
		ActivePage:     SelectPage,
		Config:         config,
		Style:          &style,
		Program:        nil,
		TerminalWidth:  80,
		TerminalHeight: 24,
	}
	return *rootContext
}

func GetRootContext() *RootContext {
	if rootContext == nil {
		panic("RootContext is not initialized")
	}
	return rootContext
}
