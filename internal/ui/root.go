package ui

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/mirror"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/ui/page"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
	"github.com/e-chan1007/hayatex/internal/utils"
)

type rootModel struct {
	tea.Model
	cfg         *config.Config
	currentPage page.BasePageModel
	pages       []page.BasePageModel
	tlpdbRef    **resolver.TLDatabase
	isCompleted bool
	Err         error
	installOnce sync.Once
}

func initRootModel(cfg *config.Config, startPage reflect.Type) *rootModel {
	tlpdb := (*resolver.TLDatabase)(nil)
	m := &rootModel{
		cfg:         cfg,
		tlpdbRef:    &tlpdb,
		isCompleted: false,
		Err:         nil,
	}
	m.cfg.Arch = utils.DetectTeXLiveArch()
	m.pages = []page.BasePageModel{
		page.NewMirrorListPage(m.cfg),
		page.NewCustomMirrorPage(m.cfg),
		page.NewSchemeListPage(m.cfg, m.tlpdbRef),
		page.NewCollectionListPage(m.cfg, m.tlpdbRef),
		page.NewCustomizeInstallPage(m.cfg),
		page.NewCustomDirectoryPage(m.cfg),
		page.NewConfirmPage(),
		page.NewInstallPage(),
	}

	if startPage != nil {
		pageIndex := slices.IndexFunc(m.pages, func(p page.BasePageModel) bool {
			return reflect.TypeOf(p) == startPage
		})
		if pageIndex != -1 {
			m.currentPage = m.pages[pageIndex]
			m.currentPage.SetPageState(page.PageStateRunning)
			return m
		}
	}
	m.currentPage = m.pages[0]
	m.currentPage.SetPageState(page.PageStateRunning)
	return m
}

func (m *rootModel) Init() tea.Cmd {
	return m.currentPage.Init()
}

func (m *rootModel) PopPage() (*rootModel, tea.Cmd) {
	currentPageIndex := slices.Index(m.pages, m.currentPage)
	if currentPageIndex > 0 {
		m.currentPage.SetPageState(page.PageStateInitial)
		m.currentPage = m.pages[currentPageIndex-1]
		m.currentPage.SetPageState(page.PageStateRunning)
	}
	return m, m.currentPage.Init()
}

func (m *rootModel) SetPage(newPage reflect.Type) (*rootModel, tea.Cmd) {
	pageIndex := slices.IndexFunc(m.pages, func(p page.BasePageModel) bool {
		return reflect.TypeOf(p) == newPage
	})
	if pageIndex != -1 {
		m.currentPage.SetPageState(page.PageStateCompleted)
		m.currentPage = m.pages[pageIndex]
		m.currentPage.SetPageState(page.PageStateRunning)
	}
	return m, m.currentPage.Init()
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.currentPage, cmd = m.currentPage.Update(msg)

	switch msg := msg.(type) {
	case tea.QuitMsg:
		return m, tea.Quit
	case page.QuitWithErrorMsg:
		m.Err = msg.Err
		return m, tea.Quit
	case page.PopPageMsg:
		return m.PopPage()
	}

	if m.currentPage.PageState() == page.PageStateCompleted {
		switch p := m.currentPage.(type) {
		case *page.MirrorListPageModel:
			if p.UseCustomMirror {
				return m.SetPage(reflect.TypeFor[*page.CustomMirrorPageModel]())
			} else {
				return m.SetPage(reflect.TypeFor[*page.SchemeListPageModel]())
			}
		case *page.CustomMirrorPageModel:
			return m.SetPage(reflect.TypeFor[*page.SchemeListPageModel]())
		case *page.SchemeListPageModel:
			if m.tlpdbRef == nil || *m.tlpdbRef == nil {
				m.Err = fmt.Errorf("tex live database is not loaded")
				return m, tea.Quit
			}
			m.cfg.SetDefaultTeXDir((*m.tlpdbRef).Year)
			if p.UseCustomCollections {
				return m.SetPage(reflect.TypeFor[*page.CollectionListPageModel]())
			}
			return m.SetPage(reflect.TypeFor[*page.CustomizeInstallPageModel]())
		case *page.CollectionListPageModel:
			return m.SetPage(reflect.TypeFor[*page.CustomizeInstallPageModel]())
		case *page.CustomizeInstallPageModel:
			if *p.CustomizeDir {
				return m.SetPage(reflect.TypeFor[*page.CustomDirectoryPageModel]())
			} else {
				return m.SetPage(reflect.TypeFor[*page.ConfirmPageModel]())
			}
		case *page.CustomDirectoryPageModel:
			return m.SetPage(reflect.TypeFor[*page.ConfirmPageModel]())
		case *page.ConfirmPageModel:
			return m.SetPage(reflect.TypeFor[*page.InstallPageModel]())
		case *page.InstallPageModel:
			if p.Err != nil {
				m.Err = p.Err
				return m, tea.Quit
			}
			m.isCompleted = true
			return m, tea.Quit
		}
	}

	if m.currentPage.PageState() == page.PageStateRunning {
		switch p := m.currentPage.(type) {
		case *page.InstallPageModel:
			m.installOnce.Do(func() {
				p.StartInstall(m.cfg, *m.tlpdbRef)
			})
		}
	}

	return m, cmd
}

func (m *rootModel) View() tea.View {
	views := []string{}
	for _, p := range m.pages {
		header := []string{}
		header = append(header, lipgloss.NewStyle().Bold(true).Render(p.Title()))
		switch p.PageState() {
		case page.PageStateInitial:
			continue

		case page.PageStateRunning:
			if desc := p.Description(); desc != "" {
				header = append(header, lipgloss.NewStyle().Foreground(styles.ColorGray()).Render(desc))
			}
			headerView := lipgloss.JoinVertical(lipgloss.Left, header...)
			views = append(views, headerView, p.View().Content)

		case page.PageStateCompleted:
			headerView := lipgloss.JoinVertical(lipgloss.Left, header...)
			switch p := p.(type) {
			case *page.InstallPageModel:
				views = append(views, headerView, p.View().Content)
			case page.BaseFormPageModel:
				views = append(views, headerView, styles.RenderAnswerText(p.DisplayValue()))
			default:
				views = append(views, headerView, p.View().Content)
			}
		default:
			fmt.Println("Unknown page state:", p.PageState())
		}
	}
	view := tea.NewView(lipgloss.NewStyle().MarginBottom(1).Render(lipgloss.JoinVertical(lipgloss.Left, views...)))
	view.AltScreen = false
	return view
}

func StartInstallWizard(cfg *config.Config, autoInstall bool) {
	startPage := reflect.TypeFor[*page.MirrorListPageModel]()
	if cfg.MirrorURL != mirror.DefaultRepositoryURL {
		startPage = reflect.TypeFor[*page.SchemeListPageModel]()
	}
	if autoInstall && cfg != nil {
		startPage = reflect.TypeFor[*page.InstallPageModel]()
	}
	var m tea.Model
	var err error
	if m, err = tea.NewProgram(initRootModel(cfg, startPage)).Run(); err != nil || m.(*rootModel).Err != nil {
		if err != nil {
			fmt.Println("Error running program:", err)
		} else {
			fmt.Println("Error running program:", m.(*rootModel).Err)
		}
		os.Exit(1)
	}
	fmt.Println()
	if !m.(*rootModel).isCompleted && m.(*rootModel).Err == nil {
		fmt.Println("Installation aborted. Exiting.")
		os.Exit(0)
	}
	if !m.(*rootModel).isCompleted && m.(*rootModel).Err != nil {
		fmt.Printf("❌️ Installation failed with error: %v\n", m.(*rootModel).Err)
		os.Exit(1)
	}
	fmt.Println("✅ Installation completed successfully!")
}
