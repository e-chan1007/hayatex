package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/ui/context"
	"github.com/e-chan1007/hayatex/internal/ui/pages"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type RootModel struct {
	context     *context.RootContext
	selectPage  tea.Model
	installPage tea.Model
}

var rootModel *RootModel

func NewRootModel(config *config.Config, db *resolver.TLDatabase) RootModel {
	ctx := context.NewRootContext(config, db)
	selectPage := pages.NewSelectPackagePage(db)
	installPage := pages.NewInstallPage(&ctx)
	rootModel = &RootModel{
		context:     &ctx,
		selectPage:  selectPage,
		installPage: installPage,
	}
	return *rootModel
}

func GetRootModel() *RootModel {
	return rootModel
}

func (m RootModel) SetProgram(program *tea.Program) {
	m.context.Program = program
}

func (m RootModel) Init() tea.Cmd {
	return nil
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case pages.SelectedSchemeMsg:
		m.context.ActivePage = context.InstallPage
		m.context.Config.RootPackages = []string{msg.Scheme}
		return m, tea.Batch(
			func() tea.Msg {
				m.context.Config.RootPackages = []string{msg.Scheme}
				return pages.StartInstallMsg{}
			},
			m.installPage.Init(),
		)

	case pages.FinishedMsg:
		if msg.Err != nil {
			fmt.Printf("Installation failed: %v\n", msg.Err)
		} else {
			fmt.Println("Installation completed successfully!")
		}

		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.context.TerminalWidth = msg.Width
		m.context.TerminalHeight = msg.Height
	}

	var cmd tea.Cmd

	switch m.context.ActivePage {
	case context.SelectPage:
		m.selectPage, cmd = m.selectPage.Update(msg)
	case context.InstallPage:
		m.installPage, cmd = m.installPage.Update(msg)
	}
	return m, cmd
}

func (m RootModel) View() tea.View {
	var view tea.View
	withHeader := true
	switch m.context.ActivePage {
	case context.SelectPage:
		view = m.selectPage.View()
	case context.InstallPage:
		view = m.installPage.View()
		withHeader = false
	default:
		view = tea.NewView("Unknown page")
	}

	if withHeader {
		view.SetContent(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Background(styles.ColorPrimary).Foreground(lipgloss.BrightWhite).Align(lipgloss.Center).Width(m.context.TerminalWidth).Bold(true).Render("Hayatex - TeX Live Installer"),
				view.Content,
			),
		)
	}
	return view
}
