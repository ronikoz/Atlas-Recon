package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"cli-tools/internal/config"
	"cli-tools/internal/runner"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type commandDef struct {
	Name           string
	Description    string
	Script         string
	RequiresTarget bool
	TargetHint     string
	ArgsHint       string
	NotImplemented bool
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
	viewport      viewport.Model
	ready         bool
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
			Name:           "scan",
			Description:    "Run nmap service scan via scan_nmap.py",
			Script:         "scan_nmap.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--ports 80,443",
		},
		{
			Name:           "dns",
			Description:    "Query DNS records via dns_lookup.py",
			Script:         "dns_lookup.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--types A,AAAA,MX",
		},
		{
			Name:           "osint",
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
			Name:           "phone",
			Description:    "Phone number parsing & dorks",
			Script:         "phone_osint.py",
			RequiresTarget: true,
			TargetHint:     "+14155552671",
			ArgsHint:       "--json",
		},
		{
			Name:           "geo",
			Description:    "Address/Coords to Map Links",
			Script:         "geo_recon.py",
			RequiresTarget: true,
			TargetHint:     "San Francisco, CA",
			ArgsHint:       "--nasa-key DEMO_KEY",
		},
		{
			Name:           "conflict",
			Description:    "Global conflict (GDELT)",
			Script:         "conflict_view.py",
			RequiresTarget: true,
			TargetHint:     "Ukraine",
			ArgsHint:       "--json",
		},
		{
			Name:           "markets",
			Description:    "Polymarket sentiment",
			Script:         "market_sentiment.py",
			RequiresTarget: true,
			TargetHint:     "Election 2024",
			ArgsHint:       "--limit 5",
		},
		{
			Name:           "social",
			Description:    "BlueSky pulse",
			Script:         "social_pulse.py",
			RequiresTarget: true,
			TargetHint:     "OSINT",
			ArgsHint:       "--limit 10",
		},
		{
			Name:           "flight",
			Description:    "OpenSky overhead flights",
			Script:         "flight_radar.py",
			RequiresTarget: true,
			TargetHint:     "Kyiv",
			ArgsHint:       "--radius 100",
		},
		{
			Name:           "war",
			Description:    "ISW Reports & Map Links",
			Script:         "war_intel.py",
			RequiresTarget: true,
			TargetHint:     "Ukraine",
			ArgsHint:       "--json",
		},
		{
			Name:           "recon",
			Description:    "crt.sh subdomain recon",
			Script:         "recon_subdomains.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--json",
		},
		{
			Name:           "web",
			Description:    "Basic web/SSL checks",
			Script:         "web_check.py",
			RequiresTarget: true,
			TargetHint:     "example.com",
			ArgsHint:       "--timeout 5",
		},
		{
			Name:        "report",
			Description: "Generate report",
			Script:      "generate_report.py",
			TargetHint:  "",
			ArgsHint:    "--title 'My Scan' --output result.md",
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
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case resultMsg:
		m = m.applyResult(msg.Result)
		// Return immediately for result updates
		return m, waitForResult(m.results)

	case tea.WindowSizeMsg:
		// Calculate fixed height of UI components
		// Header (4) + Commands (8) + Inputs (5) + Jobs (4) + Status (2) = ~23 lines
		const fixedHeight = 24
		vpHeight := msg.Height - fixedHeight
		if vpHeight < 5 {
			vpHeight = 5 // Minimum usable height
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}
		// Also update viewport with current job content on resize
		m.updateViewportContent()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cleanup()
			return m, tea.Quit
		case "tab":
			m.focusNext()
			return m, nil
		case "shift+tab":
			m.focusPrev()
			return m, nil
		case "enter":
			if m.focusIndex == 0 {
				m.focusNext()
				return m, nil
			}
			return m.runSelected()
		case "ctrl+d":
			m.showDetails = !m.showDetails
			return m, nil
		case "esc":
			m.focusIndex = 0
			return m, nil
		case "up", "k":
			if m.focusIndex == 0 || m.focusIndex == 3 {
				m = m.moveCursor(-1)
				if m.focusIndex == 3 {
					m.updateViewportContent()
				}
				return m, nil
			}
		case "down", "j":
			if m.focusIndex == 0 || m.focusIndex == 3 {
				m = m.moveCursor(1)
				if m.focusIndex == 3 {
					m.updateViewportContent()
				}
				return m, nil
			}
		}

		if m.focusIndex == 0 && msg.String() == "q" {
			m.cleanup()
			return m, tea.Quit
		}
	}

	// Update inputs
	if m.focusIndex == 1 {
		m.targetInput, cmd = m.targetInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.focusIndex == 2 {
		m.argsInput, cmd = m.argsInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport (allows scrolling with mouse or other keys if not intercepted)
	// We only pass the message to viewport if it wasn't a navigation key we handled?
	// Actually, viewport handles mouse events.
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) updateViewportContent() {
	if len(m.jobs) > 0 && m.jobCursor < len(m.jobs) {
		job := m.jobs[m.jobCursor]
		content := fmt.Sprintf("ID: %s\nStatus: %s\nDuration: %dms\n\n", job.ID, job.Status, job.Result.DurationMs)
		if job.Result.Error != "" {
			content += fmt.Sprintf("Error:\n%s\n\n", job.Result.Error)
		}
		if job.Result.Stdout != "" {
			content += fmt.Sprintf("%s\n", job.Result.Stdout)
		}
		if job.Result.Stderr != "" {
			content += fmt.Sprintf("STDERR:\n%s\n", job.Result.Stderr)
		}
		m.viewport.SetContent(content)
	} else {
		m.viewport.SetContent("Select a job to view output.")
	}
}

func (m model) View() string {
	b := &strings.Builder{}

	b.WriteString(headerStyle.Render("CLI Tools Dashboard"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab: next  enter: select/run  esc: menu  ctrl+d: details  ctrl+c: quit"))
	b.WriteString("\n\n")

	b.WriteString(m.renderCommands())
	b.WriteString("\n")
	b.WriteString(m.renderInputs())
	b.WriteString("\n")
	b.WriteString(m.renderJobs())

	if m.showDetails {
		b.WriteString("\n")
		// Use viewport view
		if !m.ready {
			b.WriteString("\n  Initializing...\n")
		} else {
			b.WriteString(m.viewport.View())
		}
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

	// Pagination logic
	const maxVisible = 5
	start := 0
	end := len(m.commands)

	if len(m.commands) > maxVisible {
		if m.menuIndex < maxVisible/2 {
			start = 0
			end = maxVisible
		} else if m.menuIndex >= len(m.commands)-maxVisible/2 {
			start = len(m.commands) - maxVisible
			end = len(m.commands)
		} else {
			start = m.menuIndex - maxVisible/2
			end = start + maxVisible
		}
	}

	for i := start; i < end; i++ {
		cmd := m.commands[i]
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

	const maxVisible = 3
	start := 0
	end := len(m.jobs)

	if len(m.jobs) > maxVisible {
		if m.jobCursor < maxVisible/2 {
			start = 0
			end = maxVisible
		} else if m.jobCursor >= len(m.jobs)-maxVisible/2 {
			start = len(m.jobs) - maxVisible
			end = len(m.jobs)
		} else {
			start = m.jobCursor - maxVisible/2
			end = start + maxVisible
		}
	}

	for i := start; i < end; i++ {
		job := m.jobs[i]
		cursor := " "
		if i == m.jobCursor {
			if m.focusIndex == 3 {
				cursor = focusStyle.Render(">")
			} else {
				cursor = ">"
			}
		}
		status := m.formatStatus(job.Status)
		line := fmt.Sprintf("%s [%s] %s", cursor, status, truncate(job.Title, 50))
		b.WriteString(line + "\n")
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

			// If this is the currently selected job, update viewport
			if i == m.jobCursor {
				m.updateViewportContent()
			}
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
