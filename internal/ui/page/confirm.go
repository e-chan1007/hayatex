package page

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type ConfirmPageModel struct {
	FormPageModel
	yes bool
}

func NewConfirmPage() *ConfirmPageModel {
	m := ConfirmPageModel{yes: true}
	return &m
}

func (m *ConfirmPageModel) Title() string {
	return "Do you want to proceed with the installation with above configuration?"
}

func (m *ConfirmPageModel) Description() string {
	return ""
}

func (m *ConfirmPageModel) DisplayValue() string {
	if m.yes {
		return "Yes"
	}
	return "No"
}

func (m *ConfirmPageModel) Init() tea.Cmd {
	return nil
}

func (m *ConfirmPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab", "left", "right":
			m.yes = !m.yes
		case "enter":
			if m.yes {
				m.SetPageState(PageStateCompleted)
				return m, nil
			} else {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *ConfirmPageModel) Button(text string, isFocused bool) string {
	btn := lipgloss.NewStyle().Padding(0, 2)
	if isFocused {
		btn = btn.Background(styles.ColorPrimary()).Bold(true)
	}
	return btn.Render(text)
}

func (m *ConfirmPageModel) View() tea.View {
	return tea.NewView(
		lipgloss.JoinHorizontal(lipgloss.Center, m.Button("Yes", m.yes), m.Button("No", !m.yes)),
	)
}
