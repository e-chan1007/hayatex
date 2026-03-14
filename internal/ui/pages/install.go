package pages

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/e-chan1007/hayatex/internal/installer"
	"github.com/e-chan1007/hayatex/internal/ui/context"
	"github.com/e-chan1007/hayatex/internal/ui/styles"
)

type InstallPage struct {
	tea.Model
	context  *context.RootContext
	program  *tea.Program
	jobs     *installer.InstallJobList
	log      string
	spinner  spinner.Model
	progress progress.Model
}

type LogLineMsg string
type JobUpdateMsg struct{ JobList *installer.InstallJobList }
type StartInstallMsg struct{}
type FinishedMsg struct{ Err error }

func NewInstallPage(context *context.RootContext) InstallPage {
	spinner := spinner.New(spinner.WithSpinner(spinner.Dot))
	spinner.Style = spinner.Style.Foreground(styles.ColorPrimary)
	return InstallPage{
		context:  context,
		jobs:     &installer.InstallJobList{},
		log:      "",
		spinner:  spinner,
		progress: progress.New(progress.WithColors(styles.ColorPrimary, styles.ColorPrimary)),
	}
}

func (page InstallPage) Init() tea.Cmd {
	return page.spinner.Tick
}

func (page InstallPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StartInstallMsg:
		go func() {
			page.StartInstall()
		}()

	case LogLineMsg:
		lines := strings.Split(page.log, "\n")
		if len(lines) >= 10 {
			lines = lines[len(lines)-9:]
		}
		page.log = strings.Join(lines, "\n") + string(msg) + "\n"

	case JobUpdateMsg:
		page.jobs = msg.JobList

	case tea.KeyMsg:
		if msg.String() == "enter" && page.jobs.AllCompleted() {
			return page, tea.Quit
		}
	}
	var cmd tea.Cmd
	page.spinner, cmd = page.spinner.Update(msg)
	return page, cmd
}

func (page InstallPage) View() tea.View {
	statusLines := []string{}

	for _, job := range page.jobs.Jobs {
		status := "⚒️"
		if job.Status == installer.InstallJobCompleted {
			status = "✅"
		} else if job.Status == installer.InstallJobExecuting {
			status = page.spinner.View()
		}

		var line string
		timeStr := job.LastUpdated.Format(time.TimeOnly)
		if job.LastUpdated.Unix() == 0 {
			timeStr = "--:--:--"
		}
		if job.HasProgress {
			line = fmt.Sprintf("%s %s %s %s", timeStr, status, job.Name, page.progress.ViewAs(job.Progress))
		} else {
			line = fmt.Sprintf("%s %s %s", timeStr, status, job.Name)
		}
		if job.Message != "" {
			line += fmt.Sprintf(" - %s", job.Message)
		}

		statusLines = append(statusLines, line)
	}

	statusLinesStr := strings.Join(statusLines, "\n")

	var footer string
	if page.jobs.AllCompleted() {
		footer = "\n\nInstallation completed successfully!"
	}

	return tea.NewView(fmt.Sprintf(
		"\nInstallation Progress:\n\n%s%s\n",
		lipgloss.NewStyle().PaddingLeft(2).BorderStyle(lipgloss.NormalBorder()).BorderLeft(true).BorderLeftForeground(lipgloss.White).Render(statusLinesStr),
		footer,
	))
}

func (page InstallPage) StartInstall() {
	jobChan := make(chan *installer.InstallJobList)
	defer close(jobChan)
	logReader, logWriter := io.Pipe()
	defer logReader.Close()
	defer logWriter.Close()

	go func() {
		scanner := bufio.NewScanner(logReader)
		for scanner.Scan() {
			page.context.Program.Send(LogLineMsg(scanner.Text()))
		}
	}()

	go func() {
		for jobList := range jobChan {
			page.context.Program.Send(JobUpdateMsg{JobList: jobList})
		}
	}()

	err := installer.Install(page.context.Config, page.context.TLDatabase, page.context.Config.RootPackages, logWriter, jobChan)
	if err != nil {
		page.context.Program.Send(LogLineMsg(fmt.Sprintf("❌ Error: %v", err)))
	}

	page.context.Program.Send(LogLineMsg("Installation completed!"))
	page.context.Program.Send(FinishedMsg{Err: err})
}
