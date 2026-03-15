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

type schemeItem struct {
	name        string
	description string
}

func (i schemeItem) FilterValue() string {
	return fmt.Sprintf("%s %s", i.name, i.description)
}

type schemeItemDelegate struct {
	styles        *styles.SelectListStyles
	maxNameLength int
}

func (d schemeItemDelegate) Height() int                             { return 1 }
func (d schemeItemDelegate) Spacing() int                            { return 0 }
func (d schemeItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d schemeItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(schemeItem)
	if !ok {
		return
	}

	fn := d.styles.Item.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return d.styles.SelectedItem.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(fmt.Sprintf("%-*s | %s", d.maxNameLength, i.name, i.description)))
}

const (
	schemeListUninitialized = iota
	schemeListLoading
	schemeListLoaded
)

type SchemeListPageModel struct {
	FormPageModel
	cfg                  *config.Config
	tlpdbRef             **resolver.TLDatabase
	list                 list.Model
	listItems            []list.Item
	listItemDelegate     schemeItemDelegate
	schemeName           string
	spinner              spinner.Model
	listState            *atomic.Int32
	UseCustomCollections bool
}

func NewSchemeListPage(cfg *config.Config, tlpdbRef **resolver.TLDatabase) *SchemeListPageModel {
	const defaultWidth = 20
	const listHeight = 14
	idelegate := schemeItemDelegate{styles: styles.NewSelectListStyles()}
	items := []list.Item{}
	l := list.New(items, idelegate, defaultWidth, listHeight)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "pick custom collections")),
		}
	}
	listState := atomic.Int32{}
	listState.Store(schemeListUninitialized)
	m := SchemeListPageModel{
		cfg:                  cfg,
		list:                 l,
		listItemDelegate:     idelegate,
		schemeName:           "",
		spinner:              styles.NewSpinner(),
		tlpdbRef:             tlpdbRef,
		listState:            &listState,
		UseCustomCollections: false,
	}
	return &m
}

func (m *SchemeListPageModel) Title() string {
	return "Which scheme do you want to use for downloading?"
}

func (m *SchemeListPageModel) Description() string {
	if m.listState.Load() == schemeListLoaded {
		return "Press 'c' to pick custom collections."
	}
	return ""
}

func (m *SchemeListPageModel) DisplayValue() string {
	return m.schemeName
}

func (m *SchemeListPageModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *SchemeListPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "c":
			if m.listState.Load() == schemeListLoaded {
				m.schemeName = "custom"
				m.UseCustomCollections = true
				m.cfg.RootPackages = []string{}
				m.SetPageState(PageStateCompleted)
				return m, nil
			}
		case "enter":
			i, ok := m.list.SelectedItem().(schemeItem)
			if ok {
				m.schemeName = i.name
				m.UseCustomCollections = false
				m.cfg.RootPackages = []string{i.name}
				m.SetPageState(PageStateCompleted)
			}
			return m, nil
		}
	}

	var listCmd, spinnerCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	m.spinner, spinnerCmd = m.spinner.Update(msg)

	if m.listState.CompareAndSwap(schemeListUninitialized, schemeListLoading) {
		return m, tea.Batch(
			spinnerCmd,
			func() tea.Msg {
				var err error
				if *m.tlpdbRef == nil {
					*m.tlpdbRef, err = resolver.RetrieveTLDatabase(m.cfg.MirrorURL)
					if err != nil {
						return QuitWithErrorMsg{Err: fmt.Errorf("failed to retrieve TL database: %w", err)}
					}
				}

				for name, scheme := range *(*m.tlpdbRef).PickByCategory("Scheme") {
					m.listItems = append(m.listItems, schemeItem{name: name, description: scheme.ShortDesc})
					m.listItemDelegate.maxNameLength = max(m.listItemDelegate.maxNameLength, len(name))
				}
				slices.SortFunc(m.listItems, func(a, b list.Item) int {
					return strings.Compare(a.(schemeItem).name, b.(schemeItem).name)
				})
				m.list.SetItems(m.listItems)
				m.list.SetDelegate(m.listItemDelegate)
				m.listState.Store(schemeListLoaded)
				return listCmd
			})
	}
	return m, tea.Batch(listCmd, spinnerCmd)
}

func (m *SchemeListPageModel) View() tea.View {
	if m.listState.Load() != schemeListLoaded {
		return tea.NewView(lipgloss.JoinHorizontal(lipgloss.Left,
			m.spinner.View(),
			"Loading TeX Live database...",
		))
	}
	return tea.NewView(m.list.View())
}
