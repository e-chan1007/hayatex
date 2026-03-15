package page

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/config"
	"github.com/e-chan1007/hayatex/internal/installer"
	"github.com/e-chan1007/hayatex/internal/mirror"
	"github.com/e-chan1007/hayatex/internal/resolver"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type InstallPageModel struct {
	FormPageModel
	cfg      *config.Config
	tlpdb    *resolver.TLDatabase
	jobList  *installer.InstallJobList
	spinner  spinner.Model
	progress progress.Model
	msgChan  chan tea.Msg
	logLines []string
	Err      error
}

type UpdateJobMsg struct {
	jobList *installer.InstallJobList
}

type LogLineMsg string

type FinishedMsg struct {
	Err error
}

func NewInstallPage() *InstallPageModel {
	m := InstallPageModel{
		jobList:  &installer.InstallJobList{},
		spinner:  styles.NewSpinner(),
		progress: styles.NewProgressBar(),
		msgChan:  make(chan tea.Msg, 100),
	}
	m.progress.SetWidth(50)
	return &m
}

func (m *InstallPageModel) Title() string {
	return "Installing TeX Live..."
}

func (m *InstallPageModel) Description() string {
	return ""
}

func (m *InstallPageModel) DisplayValue() string {
	return ""
}

func (m *InstallPageModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.waitForMessage)
}

func (m *InstallPageModel) waitForMessage() tea.Msg {
	return <-m.msgChan
}

func (m *InstallPageModel) Update(msg tea.Msg) (BasePageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case UpdateJobMsg:
		m.jobList = msg.jobList
	case LogLineMsg:
		m.logLines = append(m.logLines, string(msg))
	case FinishedMsg:
		os.WriteFile(filepath.Join(m.cfg.TexDir, "install.log"), []byte(strings.Join(m.logLines, "\n")), 0644)
		m.logLines = append(m.logLines, "Installation log saved to "+filepath.Join(m.cfg.TexDir, "install.log"))
		m.Err = msg.Err
		m.SetPageState(PageStateCompleted)
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, tea.Batch(m.waitForMessage, cmd)
}

func (m *InstallPageModel) getJobStatusSymbol(job installer.InstallJob) string {
	switch job.Status {
	case installer.InstallJobPending:
		return "⏳"
	case installer.InstallJobExecuting:
		return m.spinner.View()
	case installer.InstallJobCompleted:
		return "✅"
	case installer.InstallJobSkipped:
		return "⏩️"
	default:
		return "❓"
	}
}

func (m *InstallPageModel) installJobView() string {
	maxJobNameLen := 0
	for _, job := range m.jobList.Jobs {
		if len(job.Name) > maxJobNameLen {
			maxJobNameLen = len(job.Name)
		}
	}
	jobLines := []string{}

	for _, job := range m.jobList.Jobs {
		lastUpdate := "--:--:--"
		if job.LastUpdated.Unix() != 0 {
			lastUpdate = job.LastUpdated.Format("15:04:05")
		}
		lastUpdate = lipgloss.NewStyle().Foreground(styles.ColorGray()).Render(lastUpdate)
		if job.HasProgress {
			jobLines = append(jobLines, fmt.Sprintf("%s %s %-*s  %s %s", lastUpdate, m.getJobStatusSymbol(job), maxJobNameLen, job.Name, m.progress.ViewAs(job.Progress), job.Message))
		} else {
			jobLines = append(jobLines, fmt.Sprintf("%s %s %-*s  %s", lastUpdate, m.getJobStatusSymbol(job), maxJobNameLen, job.Name, job.Message))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, jobLines...)
}

func (m *InstallPageModel) logView() string {
	lines := m.logLines
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	return lipgloss.NewStyle().PaddingLeft(2).Foreground(styles.ColorGray()).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m *InstallPageModel) View() tea.View {
	return tea.NewView(
		lipgloss.JoinVertical(lipgloss.Left,
			m.installJobView(),
			"\nLogs:",
			m.logView(),
		),
	)
}

func (m *InstallPageModel) StartInstall(cfg *config.Config, tlpdb *resolver.TLDatabase) {
	m.cfg = cfg
	m.tlpdb = tlpdb
	go m.startInstall()
}

func (m *InstallPageModel) startInstall() {
	if m.cfg == nil {
		m.msgChan <- LogLineMsg("❌ Error: Configuration is nil")
		m.msgChan <- FinishedMsg{Err: fmt.Errorf("configuration is nil")}
		return
	}
	if m.cfg.MirrorURL == mirror.DefaultRepositoryURL {
		var err error
		m.cfg.MirrorURL, err = mirror.ResolveMirror(m.cfg.MirrorURL)
		if err != nil {
			m.msgChan <- LogLineMsg(fmt.Sprintf("❌ Error resolving mirror: %v", err))
			m.msgChan <- FinishedMsg{Err: fmt.Errorf("failed to resolve mirror: %w", err)}
			return
		}
	}
	m.msgChan <- LogLineMsg("Using mirror server: " + m.cfg.MirrorURL)
	if m.tlpdb == nil {
		var err error
		m.tlpdb, err = resolver.RetrieveTLDatabase(m.cfg.MirrorURL)
		if err != nil {
			m.msgChan <- LogLineMsg(fmt.Sprintf("❌ Error retrieving TeX Live database: %v", err))
			m.msgChan <- FinishedMsg{Err: fmt.Errorf("failed to retrieve TL database: %w", err)}
			return
		}
	}
	if m.cfg.TexDir == "" || m.cfg.TexmfLocalDir == "" {
		m.cfg.SetDefaultTeXDir(m.tlpdb.Year)
	}

	jobChan := make(chan *installer.InstallJobList)

	logReader, logWriter := io.Pipe()
	defer logReader.Close()
	defer logWriter.Close()

	go func() {
		scanner := bufio.NewScanner(logReader)
		for scanner.Scan() {
			m.msgChan <- LogLineMsg(scanner.Text())
		}
	}()

	go func() {
		for jobList := range jobChan {
			m.msgChan <- UpdateJobMsg{jobList: jobList}
		}
	}()

	err := installer.Install(m.cfg, m.tlpdb, m.cfg.RootPackages, logWriter, jobChan)
	if err != nil {
		m.msgChan <- LogLineMsg(fmt.Sprintf("❌ Error: %v", err))
	}

	m.msgChan <- LogLineMsg("Installation completed!")
	m.msgChan <- FinishedMsg{Err: err}
}
