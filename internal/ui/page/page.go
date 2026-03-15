package page

import tea "charm.land/bubbletea/v2"

const (
	PageStateInitial = iota
	PageStateRunning
	PageStateCompleted
)

type BasePageModel interface {
	Init() tea.Cmd
	Update(tea.Msg) (BasePageModel, tea.Cmd)
	View() tea.View
	PageState() int
	SetPageState(int)
	Title() string
	Description() string
}

type PageModel struct {
	BasePageModel
	state int
}

type BaseFormPageModel interface {
	BasePageModel
	DisplayValue() string
}

type FormPageModel struct {
	PageModel
	BaseFormPageModel
}

func (m *PageModel) Description() string {
	return ""
}

func (m *PageModel) PageState() int {
	return m.state
}

func (m *FormPageModel) PageState() int {
	return m.state
}

func (m *PageModel) SetPageState(state int) {
	m.state = state
}

func (m *FormPageModel) SetPageState(state int) {
	m.state = state
}

type QuitWithErrorMsg struct {
	Err error
}

type PopPageMsg struct{}

func PopPage() tea.Msg {
	return PopPageMsg{}
}
