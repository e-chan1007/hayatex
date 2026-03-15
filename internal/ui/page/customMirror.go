package page

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/mirror"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type CustomMirrorPageModel struct {
	FormPageModel
	cfg           *config.Config
	textInput     textinput.Model
	spinner       spinner.Model
	validateError error
	isValidating  bool
}

func NewCustomMirrorPage(cfg *config.Config) *CustomMirrorPageModel {
	ti := textinput.New()
	ti.SetStyles(styles.NewTextInputStyles())
	ti.SetWidth(80)
	ti.Focus()
	ti.Placeholder = "https://"

	return &CustomMirrorPageModel{cfg: cfg, textInput: ti, spinner: styles.NewSpinner()}
}

func (m *CustomMirrorPageModel) Title() string {
	return "Please enter the custom mirror URL"
}

func (m *CustomMirrorPageModel) Description() string {
	return "Press 'esc' to go back to the mirror list."
}

func (m *CustomMirrorPageModel) DisplayValue() string {
	return m.textInput.Value()
}

func (m CustomMirrorPageModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m *CustomMirrorPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	var inputCmd, spinnerCmd tea.Cmd
	m.textInput, inputCmd = m.textInput.Update(msg)
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.validateError = nil
			m.isValidating = true
			return m, tea.Batch(
				func() tea.Msg {
					err := validateURL(m.textInput.Value())
					if err == nil {
						m.cfg.MirrorURL = m.textInput.Value()
						m.SetPageState(PageStateCompleted)
					} else {
						m.validateError = err
					}
					m.isValidating = false
					return nil
				},
				spinnerCmd,
			)
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, PopPage
		}
	}
	return m, tea.Batch(inputCmd, spinnerCmd)
}

func (m *CustomMirrorPageModel) View() tea.View {
	items := []string{m.textInput.View()}
	if m.validateError != nil {
		items = append(items, styles.RenderErrorText(m.validateError.Error()))
	}
	if m.isValidating {
		items = append(items, lipgloss.JoinHorizontal(lipgloss.Center, m.spinner.View(), "Checking URL..."))
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, items...))
}

func validateURL(url string) error {
	err := mirror.ValidateMirrorURL(url)
	if err != nil {
		return fmt.Errorf("invalid mirror URL: %w", err)
	}
	return nil
}
