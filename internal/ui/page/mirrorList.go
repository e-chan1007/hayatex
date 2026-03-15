package page

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/mirror"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type mirrorItem struct {
	mirror.Mirror
}

func (i mirrorItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s", i.URL, i.Region, i.Country)
}

type mirrorItemDelegate struct {
	styles           *styles.SelectListStyles
	maxRegionLength  int
	maxCountryLength int
	maxURLLength     int
}

func (d mirrorItemDelegate) Height() int                             { return 1 }
func (d mirrorItemDelegate) Spacing() int                            { return 0 }
func (d mirrorItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d mirrorItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(mirrorItem)
	if !ok {
		return
	}

	fn := d.styles.Item.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return d.styles.SelectedItem.Render("> " + strings.Join(s, " "))
		}
	}

	if i.IsPreferred {
		fmt.Fprint(w, fn(fmt.Sprintf("%-*s | %-*s | %s - preferred", d.maxRegionLength, i.Region, d.maxCountryLength, i.Country, i.URL)))
	} else {
		fmt.Fprint(w, fn(fmt.Sprintf("%-*s | %-*s | %s", d.maxRegionLength, i.Region, d.maxCountryLength, i.Country, i.URL)))
	}
}

type MirrorListPageModel struct {
	FormPageModel
	list            list.Model
	cfg             *config.Config
	mirrorURL       string
	UseCustomMirror bool
}

func NewMirrorListPage(cfg *config.Config) *MirrorListPageModel {
	idelegate := mirrorItemDelegate{styles: styles.NewSelectListStyles()}

	mirrors := mirror.GetMirrorList()
	items := []list.Item{}
	for _, mirror := range mirrors {
		items = append(items, mirrorItem{Mirror: mirror})
		idelegate.maxRegionLength = max(idelegate.maxRegionLength, len(mirror.Region))
		idelegate.maxCountryLength = max(idelegate.maxCountryLength, len(mirror.Country))
		idelegate.maxURLLength = max(idelegate.maxURLLength, len(mirror.URL))
	}

	const defaultWidth = 20
	const listHeight = 14

	l := list.New(items, idelegate, defaultWidth, listHeight)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.Select(slices.IndexFunc(mirrors, func(m mirror.Mirror) bool { return m.IsPreferred }))
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "select mirror")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "use custom URL")),
		}
	}

	m := MirrorListPageModel{list: l, cfg: cfg, UseCustomMirror: false}
	return &m
}

func (m *MirrorListPageModel) Title() string {
	return "Which CTAN mirror would you like to use?"
}

func (m *MirrorListPageModel) Description() string {
	return "Use the arrow keys to navigate the list. Press 'c' to enter a custom mirror URL."
}

func (m *MirrorListPageModel) DisplayValue() string {
	return m.mirrorURL
}

func (m *MirrorListPageModel) Init() tea.Cmd {
	return nil
}

func (m *MirrorListPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "c":
			m.mirrorURL = "custom"
			m.cfg.MirrorURL = ""
			m.UseCustomMirror = true
			m.SetPageState(PageStateCompleted)
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			i, ok := m.list.SelectedItem().(mirrorItem)
			if ok {
				m.mirrorURL = i.URL
				m.cfg.MirrorURL = i.URL
				m.UseCustomMirror = false
				m.SetPageState(PageStateCompleted)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *MirrorListPageModel) View() tea.View {
	return tea.NewView(m.list.View())
}
