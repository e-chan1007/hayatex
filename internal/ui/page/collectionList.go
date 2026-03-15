package page

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type collectionItem struct {
	name        string
	description string
	isSelected  bool
}

func (i collectionItem) FilterValue() string {
	return fmt.Sprintf("%s %s", i.name, i.description)
}

type collectionItemDelegate struct {
	styles        *styles.SelectListStyles
	maxNameLength int
}

func (d collectionItemDelegate) Height() int                             { return 1 }
func (d collectionItemDelegate) Spacing() int                            { return 0 }
func (d collectionItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d collectionItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(collectionItem)
	if !ok {
		return
	}

	fn := d.styles.Item.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return d.styles.SelectedItem.Render("> " + strings.Join(s, " "))
		}
	}

	if i.isSelected {
		fmt.Fprint(w, fn(fmt.Sprintf("[x] %-*s | %s", d.maxNameLength, i.name, i.description)))
	} else {
		fmt.Fprint(w, fn(fmt.Sprintf("[ ] %-*s | %s", d.maxNameLength, i.name, i.description)))
	}
}

const (
	collectionListUninitialized = iota
	collectionListLoading
	collectionListLoaded
)

type CollectionListPageModel struct {
	FormPageModel
	cfg              *config.Config
	tlpdbRef         **resolver.TLDatabase
	list             list.Model
	listItems        []list.Item
	listItemDelegate collectionItemDelegate
	collectionNames  []string
	spinner          spinner.Model
	listState        *atomic.Int32
	validateError    error
}

func NewCollectionListPage(cfg *config.Config, tlpdbRef **resolver.TLDatabase) *CollectionListPageModel {
	const defaultWidth = 20
	const listHeight = 14
	idelegate := collectionItemDelegate{styles: styles.NewSelectListStyles()}
	items := []list.Item{}
	l := list.New(items, idelegate, defaultWidth, listHeight)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select/deselect collection")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm selection")),
		}
	}
	listState := atomic.Int32{}
	listState.Store(collectionListUninitialized)
	m := CollectionListPageModel{
		cfg:              cfg,
		list:             l,
		listItemDelegate: idelegate,
		collectionNames:  []string{},
		spinner:          styles.NewSpinner(),
		tlpdbRef:         tlpdbRef,
		listState:        &listState,
		validateError:    nil,
	}
	return &m
}

func (m *CollectionListPageModel) Title() string {
	return "Which collection do you want to use for downloading?"
}

func (m *CollectionListPageModel) Description() string {
	return ""
}

func (m *CollectionListPageModel) DisplayValue() string {
	if len(m.collectionNames) == 0 {
		return ""
	}
	return strings.Join(m.collectionNames, ", ")
}

func (m *CollectionListPageModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *CollectionListPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "space":
			i, ok := m.list.SelectedItem().(collectionItem)
			if ok {
				i.isSelected = !i.isSelected
			}
			m.listItems[m.list.Index()] = i
			m.list.SetItems(m.listItems)
			return m, nil
		case "enter":
			m.collectionNames = []string{}
			for _, item := range m.listItems {
				collectionItem, _ := item.(collectionItem)
				if collectionItem.isSelected {
					m.collectionNames = append(m.collectionNames, collectionItem.name)
				}
			}
			if len(m.collectionNames) == 0 {
				m.validateError = fmt.Errorf("please select at least one collection")
				return m, nil
			}
			m.cfg.RootPackages = m.collectionNames
			m.SetPageState(PageStateCompleted)
			return m, nil
		}
	}

	var listCmd, spinnerCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	m.spinner, spinnerCmd = m.spinner.Update(msg)

	if m.listState.CompareAndSwap(collectionListUninitialized, collectionListLoading) {
		return m, tea.Batch(
			spinnerCmd,
			func() tea.Msg {
				for name, collection := range *(*m.tlpdbRef).PickByCategory("Collection") {
					m.listItems = append(m.listItems, collectionItem{name: name, description: collection.ShortDesc})
					m.listItemDelegate.maxNameLength = max(m.listItemDelegate.maxNameLength, len(name))
				}
				slices.SortFunc(m.listItems, func(a, b list.Item) int {
					return strings.Compare(a.(collectionItem).name, b.(collectionItem).name)
				})
				m.list.SetItems(m.listItems)
				m.list.SetDelegate(m.listItemDelegate)
				m.listState.Store(collectionListLoaded)
				return listCmd
			})
	}
	return m, tea.Batch(listCmd, spinnerCmd)
}

func (m *CollectionListPageModel) View() tea.View {
	if m.listState.Load() != collectionListLoaded {
		return tea.NewView(lipgloss.JoinHorizontal(lipgloss.Left,
			m.spinner.View(),
			"Loading TeX Live database...",
		))
	}
	if m.validateError != nil {
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, m.list.View(), styles.RenderErrorText(m.validateError.Error())))
	}
	return tea.NewView(m.list.View())
}
