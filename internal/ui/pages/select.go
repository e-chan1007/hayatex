package pages

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/ui/context"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type SelectPackagePage struct {
	tea.Model
	context *context.RootContext
	list    list.Model
}

type SchemeItem struct {
	pkg *resolver.TLPackage
}

type SelectedSchemeMsg struct {
	Scheme string
}

func (i SchemeItem) Title() string       { return i.pkg.Name }
func (i SchemeItem) Description() string { return i.pkg.ShortDesc }
func (i SchemeItem) FilterValue() string { return i.pkg.Name }

func NewSelectPackagePage(tlpdb *resolver.TLDatabase) SelectPackagePage {
	schemes := tlpdb.PickByCategory("Scheme")
	var items []list.Item
	for _, pkg := range *schemes {
		items = append(items, SchemeItem{pkg: pkg})
	}
	slices.SortFunc(items, func(a, b list.Item) int {
		return strings.Compare(a.FilterValue(), b.FilterValue())
	})
	ctx := context.GetRootContext()
	ld := list.NewDefaultDelegate()
	ld.Styles.SelectedTitle = ld.Styles.SelectedTitle.Foreground(styles.ColorPrimary).Bold(true).BorderLeftForeground(styles.ColorPrimary)
	ld.Styles.SelectedDesc = ld.Styles.SelectedDesc.Foreground(styles.ColorPrimary).BorderLeftForeground(styles.ColorPrimary)
	l := list.New(items, ld, 0, 0)
	l.Title = "Select TeX Live Scheme"
	l.Styles.TitleBar = *ctx.Style
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().Padding(0, 2, 0, 2).Foreground(lipgloss.White).Background(styles.ColorPrimary)
	return SelectPackagePage{
		context: ctx,
		list:    l,
	}
}

func (page SelectPackagePage) Init() tea.Cmd { return nil }

func (page SelectPackagePage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			selected := page.SelectedScheme()
			return page, func() tea.Msg {
				return SelectedSchemeMsg{Scheme: selected}
			}
		}
	case tea.WindowSizeMsg:
		h, v := page.context.Style.GetFrameSize()
		page.list.SetSize(msg.Width-h, msg.Height-v-1)
	}

	var cmd tea.Cmd
	page.list, cmd = page.list.Update(msg)
	return page, cmd
}

func (page SelectPackagePage) View() tea.View {
	return tea.NewView(page.list.View())
}

func (page SelectPackagePage) SelectedScheme() string {
	if i, ok := page.list.SelectedItem().(interface{ Title() string }); ok {
		return i.Title()
	}
	return ""
}
