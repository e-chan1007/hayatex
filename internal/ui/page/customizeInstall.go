package page

import (
	"fmt"
	"io"
	"strings"

	"atomicgo.dev/isadmin"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type customizeItem struct {
	name        string
	description string
	isSelected  *bool
	disabled    bool
}

func (i customizeItem) FilterValue() string {
	return fmt.Sprintf("%s %s", i.name, i.description)
}

type customizeItemDelegate struct {
	styles        *styles.SelectListStyles
	maxNameLength int
}

func (d customizeItemDelegate) Height() int                             { return 1 }
func (d customizeItemDelegate) Spacing() int                            { return 0 }
func (d customizeItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d customizeItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(customizeItem)
	if !ok {
		return
	}

	fn := d.styles.Item.Render
	if i.disabled {
		fn = d.styles.DisabledItem.Render
	}
	if index == m.Index() {
		fn = func(s ...string) string {
			if i.disabled {
				return d.styles.DisabledItem.PaddingLeft(2).Render("> " + strings.Join(s, " "))
			}
			return d.styles.SelectedItem.Render("> " + strings.Join(s, " "))
		}
	}

	if *i.isSelected {
		fmt.Fprint(w, fn(fmt.Sprintf("[x] %-*s (%s)", d.maxNameLength, i.name, i.description)))
	} else {
		fmt.Fprint(w, fn(fmt.Sprintf("[ ] %-*s (%s)", d.maxNameLength, i.name, i.description)))
	}
}

type CustomizeInstallPageModel struct {
	FormPageModel
	cfg              *config.Config
	list             list.Model
	listItems        []list.Item
	listItemDelegate customizeItemDelegate
	customizeNames   []string
	spinner          spinner.Model
	CustomizeDir     *bool
}

func NewCustomizeInstallPage(cfg *config.Config) *CustomizeInstallPageModel {
	const defaultWidth = 20
	const listHeight = 5
	idelegate := customizeItemDelegate{styles: styles.NewSelectListStyles()}
	customizeDir := false
	items := []list.Item{}
	l := list.New(items, idelegate, defaultWidth, listHeight)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select/deselect customize")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm selection")),
		}
	}
	m := CustomizeInstallPageModel{
		cfg:              cfg,
		list:             l,
		listItems:        items,
		listItemDelegate: idelegate,
		customizeNames:   []string{},
		spinner:          styles.NewSpinner(),
		CustomizeDir:     &customizeDir,
	}
	return &m
}

func (m *CustomizeInstallPageModel) Title() string {
	return "Which customize do you want to use for installation?"
}

func (m *CustomizeInstallPageModel) Description() string {
	return ""
}

func (m *CustomizeInstallPageModel) DisplayValue() string {
	customizeList := []string{}
	for _, item := range m.listItems {
		customizeItem, _ := item.(customizeItem)
		if *customizeItem.isSelected {
			customizeList = append(customizeList, customizeItem.name)
		}
	}
	if !*m.CustomizeDir {
		customizeList = append(customizeList, fmt.Sprintf("Install to: %s", m.cfg.TexDir))
	}
	return strings.Join(customizeList, ", ")
}

func (m *CustomizeInstallPageModel) Init() tea.Cmd {
	isAdmin := isadmin.Check()
	items := []list.Item{
		customizeItem{name: "Add to PATH", description: "Add TeX Live binaries to PATH environment variable", isSelected: &m.cfg.AddPath},
		customizeItem{name: "Install Documentation", description: "Install documentation files for TeX Live packages", isSelected: &m.cfg.InstallDocFiles},
		customizeItem{name: "Install Source code", description: "Install source files for TeX Live packages", isSelected: &m.cfg.InstallSrcFiles},
		customizeItem{name: "Install for All Users", description: "Install TeX Live for all users (requires administrator privileges)", isSelected: &m.cfg.InstallForAllUsers, disabled: !isAdmin},
		customizeItem{name: "Change Installation Directory", description: fmt.Sprintf("Install TeX Live to a custom directory (Current: %s)", m.cfg.TexDir), isSelected: m.CustomizeDir},
	}
	m.listItems = items
	m.list.SetItems(m.listItems)
	m.list.SetHeight(5 + len(items))
	return m.spinner.Tick
}

func (m *CustomizeInstallPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "space":
			i, ok := m.list.SelectedItem().(customizeItem)
			if i.disabled {
				return m, nil
			}
			if ok {
				*i.isSelected = !*i.isSelected
			}
			m.listItems[m.list.Index()] = i
			m.list.SetItems(m.listItems)
			return m, nil
		case "enter":
			m.customizeNames = []string{}
			for _, item := range m.listItems {
				customizeItem, _ := item.(customizeItem)
				if *customizeItem.isSelected {
					m.customizeNames = append(m.customizeNames, customizeItem.name)
				}
			}
			m.SetPageState(PageStateCompleted)
			return m, nil
		}
	}

	var listCmd, spinnerCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	return m, tea.Batch(listCmd, spinnerCmd)
}

func (m *CustomizeInstallPageModel) View() tea.View {
	return tea.NewView(m.list.View())
}
