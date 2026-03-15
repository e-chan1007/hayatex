package page

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
	"github.com/e-chan1007/hayatex/internal/utils"
)

type CustomDirectoryPageModel struct {
	FormPageModel
	cfg                *config.Config
	texLiveDirInput    textinput.Model
	texMfLocalDirInput textinput.Model
	texLiveDirError    error
	texMfLocalDirError error
}

func NewCustomDirectoryPage(cfg *config.Config) *CustomDirectoryPageModel {
	texLiveDirInput := textinput.New()
	texLiveDirInput.Prompt = "TeX Live Installation Directory(TEXDIR): "
	texLiveDirInput.Focus()
	texLiveDirInput.SetStyles(styles.NewTextInputStyles())
	texLiveDirInput.SetWidth(50)

	texMfLocalDirInput := textinput.New()
	texMfLocalDirInput.Prompt = fmt.Sprintf("%*s", len(texLiveDirInput.Prompt), "TeX Live Local Directory(TEXMFLOCAL): ")
	texMfLocalDirInput.SetStyles(styles.NewTextInputStyles())
	texMfLocalDirInput.SetWidth(50)

	return &CustomDirectoryPageModel{cfg: cfg, texLiveDirInput: texLiveDirInput, texMfLocalDirInput: texMfLocalDirInput}
}

func (m *CustomDirectoryPageModel) Title() string {
	return "Please enter the custom directory"
}

func (m *CustomDirectoryPageModel) Description() string {
	return "Press 'tab' to switch between inputs. Press 'enter' to confirm."
}

func (m *CustomDirectoryPageModel) DisplayValue() string {
	return fmt.Sprintf("TEXDIR: %s, TEXMFLOCAL: %s", m.texLiveDirInput.Value(), m.texMfLocalDirInput.Value())
}

func (m *CustomDirectoryPageModel) Init() tea.Cmd {
	m.texLiveDirInput.SetValue(m.cfg.TexDir)
	m.texMfLocalDirInput.SetValue(m.cfg.TexmfLocalDir)
	return tea.Batch(textinput.Blink)
}

func (m *CustomDirectoryPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	var texLiveDirInputCmd, texMfLocalDirInputCmd tea.Cmd
	m.texLiveDirInput, texLiveDirInputCmd = m.texLiveDirInput.Update(msg)
	m.texMfLocalDirInput, texMfLocalDirInputCmd = m.texMfLocalDirInput.Update(msg)
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			if m.texLiveDirInput.Focused() {
				m.texLiveDirInput.Blur()
				m.texMfLocalDirInput.Focus()
			} else {
				m.texLiveDirInput.Focus()
				m.texMfLocalDirInput.Blur()
			}
		case "enter":
			m.texLiveDirError = nil
			m.texMfLocalDirError = nil
			return m, func() tea.Msg {
				texLiveDir := m.texLiveDirInput.Value()
				texMfLocalDir := m.texMfLocalDirInput.Value()
				m.texLiveDirError = utils.CheckWritePermission(texLiveDir)
				m.texMfLocalDirError = utils.CheckWritePermission(texMfLocalDir)
				if m.texLiveDirError == nil && m.texMfLocalDirError == nil {
					m.cfg.TexDir = texLiveDir
					m.cfg.TexmfLocalDir = texMfLocalDir
					m.SetPageState(PageStateCompleted)
				}
				return nil
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, tea.Batch(texLiveDirInputCmd, texMfLocalDirInputCmd)
}

func (m *CustomDirectoryPageModel) View() tea.View {
	items := []string{}
	items = append(items, m.texLiveDirInput.View())
	if m.texLiveDirError != nil {
		items = append(items, styles.RenderErrorText(m.texLiveDirError.Error()))
	} else {
		items = append(items, "")
	}
	items = append(items, m.texMfLocalDirInput.View())
	if m.texMfLocalDirError != nil {
		items = append(items, styles.RenderErrorText(m.texMfLocalDirError.Error()))
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, items...))
}
