package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"cli-tools/internal/config"
	"cli-tools/internal/runner"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type commandDef struct {
	Name            string
	Description     string
	Script          string
	RequiresTarget  bool
	TargetHint      string
	ArgsHint        string
	NotImplemented  bool
}

type uiJob struct {
	ID     string
	Title  string
	Status string
	Result runner.Result
}

type resultMsg struct {
	Result runner.Result
}

type resultsClosedMsg struct{}

type model struct {
	cfg           config.Config
	commands      []commandDef
	menuIndex     int
	focusIndex    int
	targetInput   textinput.Model
	argsInput     textinput.Model
	jobs          []uiJob
	jobCursor     int
	showDetails   bool
	queue         *runner.Queue
	results       <-chan runner.Result
	cancel        context.CancelFunc
	statusMessage string
}

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	focusStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	statusQueued = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	statusRun    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	statusFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func Run(cfg config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	q := runner.NewQueue(cfg.Concurrency)
	q.Start(ctx)

	m := newModel(cfg, q, cancel)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(cfg config.Config, q *runner.Queue, cancel context.CancelFunc) model {
	commands := []commandDef{
		{
			Name:           "scan (nmap)",
			Description:    "Run nmap service scan via scan_nmap.py",
			Script:         "scan_nmap.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--ports 80,443",
		},
		{
			Name:           "dns lookup",
			Description:    "Query DNS records via dns_lookup.py",
			Script:         "dns_lookup.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--types A,AAAA,MX",
		},
		{
			Name:           "osint domain",
			Description:    "crt.sh + whois + DNS enrichment",
			Script:         "osint_domain.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--enrich-dns",
		},
		{
			Name:        "osint suite",
			Description: "Multi-source OSINT suite",
			Script:      "osint_suite.py",
			TargetHint:  "",
			ArgsHint:    "--category core --username alice",
		},
		{
			Name:           "recon subdomains",
			Description:    "crt.sh subdomain recon",
			Script:         "recon_subdomains.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--json",
		},
		{
			Name:           "web",
			Description:    "Web checks (not implemented)",
			NotImplemented: true,
		},
		{
			Name:           "report",
			Description:    "Reporting (not implemented)",
			NotImplemented: true,
		},
	}

	target := textinput.New()
	target.Placeholder = commands[0].TargetHint
	target.Prompt = "Target: "
	target.CharLimit = 256
	target.Width = 40

	args := textinput.New()
	args.Placeholder = commands[0].ArgsHint
	args.Prompt = "Args:   "
	args.CharLimit = 512
	args.Width = 60

	return model{
		cfg:         cfg,
		commands:    commands,
		menuIndex:   0,
		focusIndex:  0,
		targetInput: target,
		argsInput:   args,
		jobs:        []uiJob{},
		jobCursor:   0,
		showDetails: true,
		queue:       q,
		results:     q.Results(),
		cancel:      cancel,
	}
}

func (m model) Init() tea.Cmd {
	return waitForResult(m.results)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case resultMsg:
		m = m.applyResult(msg.Result)
		return m, waitForResult(m.results)
	case resultsClosedMsg:
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cleanup()
			return m, tea.Quit
		case "tab":
			m.focusNext()
			return m, nil
		case "shift+tab":
			m.focusPrev()
			return m, nil
		case "r":
			return m.runSelected()
		case "d":
			m.showDetails = !m.showDetails
			return m, nil
		case "up", "k":
			m = m.moveCursor(-1)
			return m, nil
		case "down", "j":
			m = m.moveCursor(1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.focusIndex {
	case 1:
		m.targetInput, cmd = m.targetInput.Update(msg)
	case 2:
		m.argsInput, cmd = m.argsInput.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	b := &strings.Builder{}

	b.WriteString(headerStyle.Render("CLI Tools Dashboard"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab/shift+tab: move  r: run  d: toggle details  q: quit"))
	b.WriteString("\n\n")

	b.WriteString(m.renderCommands())
	b.WriteString("\n")
	b.WriteString(m.renderInputs())
	b.WriteString("\n")
	b.WriteString(m.renderJobs())

	if m.showDetails {
		b.WriteString("\n")
		b.WriteString(m.renderDetails())
	}

	if m.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(m.statusMessage))
	}

	return b.String()
}

func (m *model) cleanup() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.queue != nil {
		m.queue.Stop()
	}
}

func (m model) renderCommands() string {
	b := &strings.Builder{}
	b.WriteString("Commands:\n")
	for i, cmd := range m.commands {
		cursor := " "
		if i == m.menuIndex {
			if m.focusIndex == 0 {
				cursor = focusStyle.Render(">")
			} else {
				cursor = ">"
			}
		}
		label := cmd.Name
		if cmd.NotImplemented {
			label += " (disabled)"
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, label))
	}

	selected := m.commands[m.menuIndex]
	b.WriteString("\nSelected: ")
	b.WriteString(selected.Name)
	b.WriteString(" - ")
	b.WriteString(selected.Description)
	b.WriteString("\n")
	return b.String()
}

func (m model) renderInputs() string {
	b := &strings.Builder{}
	b.WriteString("Inputs:\n")
	b.WriteString(m.renderInput(m.targetInput, 1))
	b.WriteString("\n")
	b.WriteString(m.renderInput(m.argsInput, 2))
	b.WriteString("\n")
	return b.String()
}

func (m model) renderInput(input textinput.Model, idx int) string {
	view := input.View()
	if m.focusIndex == idx {
		return focusStyle.Render(view)
	}
	return view
}

func (m model) renderJobs() string {
	b := &strings.Builder{}
	b.WriteString("Jobs:\n")
	if len(m.jobs) == 0 {
		b.WriteString("  (no jobs yet)\n")
		return b.String()
	}

	for i, job := range m.jobs {
		cursor := " "
		if i == m.jobCursor {
			if m.focusIndex == 3 {
				cursor = focusStyle.Render(">")
			} else {
				cursor = ">"
			}
		}
		status := m.formatStatus(job.Status)
		line := fmt.Sprintf("%s [%s] %s", cursor, status, truncate(job.Title, 60))
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m model) renderDetails() string {
	if len(m.jobs) == 0 {
		return ""
	}
	job := m.jobs[m.jobCursor]
	b := &strings.Builder{}
	b.WriteString("Details:\n")
	b.WriteString(fmt.Sprintf("  ID: %s\n", job.ID))
	b.WriteString(fmt.Sprintf("  Status: %s\n", job.Status))
	if job.Result.DurationMs > 0 {
		b.WriteString(fmt.Sprintf("  Duration: %dms\n", job.Result.DurationMs))
	}
	stdout := strings.TrimSpace(job.Result.Stdout)
	stderr := strings.TrimSpace(job.Result.Stderr)
	if stdout != "" {
		b.WriteString("\n  Stdout (tail):\n")
		b.WriteString(indent(tailLines(stdout, 20), "  "))
		b.WriteString("\n")
	}
	if stderr != "" {
		b.WriteString("\n  Stderr (tail):\n")
		b.WriteString(indent(tailLines(stderr, 20), "  "))
		b.WriteString("\n")
	}
	if job.Result.Error != "" {
		b.WriteString("\n  Error:\n")
		b.WriteString(indent(job.Result.Error, "  "))
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) formatStatus(status string) string {
	switch status {
	case "queued":
		return statusQueued.Render(status)
	case "running":
		return statusRun.Render(status)
	case "done":
		return statusOK.Render(status)
	case "failed":
		return statusFail.Render(status)
	default:
		return status
	}
}

func (m model) moveCursor(delta int) model {
	if m.focusIndex == 0 {
		m.menuIndex = clamp(m.menuIndex+delta, 0, len(m.commands)-1)
		m.syncPlaceholders()
		return m
	}
	if m.focusIndex == 3 {
		m.jobCursor = clamp(m.jobCursor+delta, 0, len(m.jobs)-1)
		return m
	}
	return m
}

func (m *model) focusNext() {
	m.focusIndex = (m.focusIndex + 1) % 4
	m.applyFocus()
}

func (m *model) focusPrev() {
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = 3
	}
	m.applyFocus()
}

func (m *model) applyFocus() {
	m.targetInput.Blur()
	m.argsInput.Blur()
	if m.focusIndex == 1 {
		m.targetInput.Focus()
	}
	if m.focusIndex == 2 {
		m.argsInput.Focus()
	}
}

func (m *model) syncPlaceholders() {
	cmd := m.commands[m.menuIndex]
	m.targetInput.Placeholder = cmd.TargetHint
	m.argsInput.Placeholder = cmd.ArgsHint
}

func (m model) runSelected() (tea.Model, tea.Cmd) {
	cmdDef := m.commands[m.menuIndex]
	if cmdDef.NotImplemented {
		m.statusMessage = "selected command is not implemented"
		return m, nil
	}

	target := strings.TrimSpace(m.targetInput.Value())
	args := strings.TrimSpace(m.argsInput.Value())
	if cmdDef.RequiresTarget && target == "" {
		m.statusMessage = "target is required"
		return m, nil
	}

	argList := []string{}
	if cmdDef.RequiresTarget {
		argList = append(argList, target)
	}
	if args != "" {
		argList = append(argList, strings.Fields(args)...)
	}

	id := fmt.Sprintf("dash-%d", time.Now().UnixNano())
	jobTitle := cmdDef.Name
	if target != "" {
		jobTitle += " " + target
	}
	job := uiJob{ID: id, Title: jobTitle, Status: "running"}
	m.jobs = append(m.jobs, job)
	m.jobCursor = len(m.jobs) - 1
	m.statusMessage = fmt.Sprintf("queued job: %s", jobTitle)

	script := pluginPath(cmdDef.Script)
	m.queue.Submit(runner.Job{
		ID:      id,
		Command: cmdDef.Name,
		Args:    argList,
		Run: func(ctx context.Context) (runner.Result, error) {
			result, err := runner.RunPython(script, argList, runner.RunOptions{
				Stream: false,
				Python: m.cfg.Paths.Python,
			})
			result.ID = id
			return result, err
		},
	})

	return m, nil
}

func (m model) applyResult(result runner.Result) model {
	for i := range m.jobs {
		if m.jobs[i].ID == result.ID {
			m.jobs[i].Result = result
			if result.Status == runner.StatusSuccess {
				m.jobs[i].Status = "done"
			} else {
				m.jobs[i].Status = "failed"
			}
			m.statusMessage = fmt.Sprintf("job finished: %s", m.jobs[i].Title)
			break
		}
	}
	return m
}

func waitForResult(ch <-chan runner.Result) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-ch
		if !ok {
			return resultsClosedMsg{}
		}
		return resultMsg{Result: res}
	}
}

func pluginPath(name string) string {
	if name == "" {
		return ""
	}
	return filepath.Join("plugins", "python", name)
}

func clamp(val int, min int, max int) int {
	if max < min {
		return min
	}
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func tailLines(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func indent(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func truncate(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit < 3 {
		return text[:limit]
	}
	return text[:limit-3] + "..."
}


// Signed-off-by: ronikoz
