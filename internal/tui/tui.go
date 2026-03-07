package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ronikoz/atlas-recon/internal/config"
	"github.com/ronikoz/atlas-recon/internal/plugins"
	"github.com/ronikoz/atlas-recon/internal/runner"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/shlex"
)

type commandDef struct {
	Name           string
	Description    string
	Script         string
	RequiresTarget bool
	TargetHint     string
	ArgsHint       string
	ArgsOptions    []string
	NotImplemented bool
}

type uiJob struct {
	ID     string
	Title  string
	Status string
	Result runner.Result
	Cancel context.CancelFunc
}

type resultMsg struct {
	Result runner.Result
}

type resultsClosedMsg struct{}

type model struct {
	cfg           config.Config
	allCommands   []commandDef
	commands      []commandDef
	menuIndex     int
	focusIndex    int
	targetInput   textinput.Model
	argsInput     textinput.Model
	filterInput   textinput.Model
	isFiltering   bool
	jobs          []uiJob
	jobCursor     int
	showDetails   bool
	queue         *runner.Queue
	results       <-chan runner.Result
	cancel        context.CancelFunc
	statusMessage string
	viewport      viewport.Model
	ready         bool
	spinner       spinner.Model
	showHelp      bool
	width         int
	height        int
}

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	focusStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	statusQueued = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	statusRun    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	statusFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	borderStyle  = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(lipgloss.Color("238")).PaddingLeft(2)
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
		// Note: TUI scan runs the embedded scan_nmap.py plugin.
		// The native Go scanner is only available via the CLI (ct scan).
		{
			Name:           "scan",
			Description:    "TCP port scan (embedded plugin)",
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
			Name:           "osint suite",
			Description:    "Multi-source OSINT (Arrow keys to cycle category)",
			Script:         "osint_suite.py",
			RequiresTarget: true,
			TargetHint:     "domain/user/ip",
			ArgsHint:       "--username alice",
			ArgsOptions: []string{
				"--category core",
				"--category social",
				"--category domain_dns",
				"--category ip_infra",
				"--category metadata",
				"--category leaks",
				"--category archives",
				"--category search",
				"--category threat",
			},
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

	// Add dynamically found plugins
	foundPlugins, _ := plugins.ListPlugins()
	for _, p := range foundPlugins {
		exists := false
		for _, c := range commands {
			if c.Script == p {
				exists = true
				break
			}
		}
		if !exists {
			name := strings.TrimSuffix(p, ".py")
			commands = append(commands, commandDef{
				Name:           name,
				Description:    "Dynamically loaded script",
				Script:         p,
				RequiresTarget: true,
				TargetHint:     "target",
				ArgsHint:       "--help",
			})
		}
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

	filter := textinput.New()
	filter.Placeholder = "Filter commands..."
	filter.Prompt = "/ "
	filter.Width = 30

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		cfg:         cfg,
		allCommands: commands,
		commands:    commands,
		menuIndex:   0,
		focusIndex:  0,
		targetInput: target,
		argsInput:   args,
		filterInput: filter,
		jobs:        []uiJob{},
		jobCursor:   0,
		showDetails: true,
		queue:       q,
		results:     q.Results(),
		cancel:      cancel,
		spinner:     s,
		width:       80,
		height:      24,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(waitForResult(m.results), m.spinner.Tick)
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

	case resultsClosedMsg:
		m.cleanup()
		return m, tea.Quit

	case spinner.TickMsg:
		var sCmd tea.Cmd
		m.spinner, sCmd = m.spinner.Update(msg)
		cmds = append(cmds, sCmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		vpWidth := msg.Width
		if msg.Width > 100 {
			vpWidth = msg.Width - (msg.Width * 45 / 100) - 4
		}

		vpHeight := msg.Height - 24
		if msg.Width > 100 {
			vpHeight = msg.Height - 6 // Header + Footer
		}
		if vpHeight < 5 {
			vpHeight = 5
		}

		if !m.ready {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}
		m.updateViewportContent()

	case tea.KeyMsg:
		if m.isFiltering {
			switch msg.String() {
			case "enter", "esc":
				m.isFiltering = false
				m.filterInput.Blur()
				return m, nil
			}
			m.filterInput, cmd = m.filterInput.Update(msg)
			cmds = append(cmds, cmd)
			m.filterCommands()
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "ctrl+c":
			m.cleanup()
			return m, tea.Quit
		case "?":
			if m.focusIndex == 0 || m.focusIndex == 3 {
				m.showHelp = !m.showHelp
				return m, nil
			}
		case "/":
			if m.focusIndex == 0 {
				m.isFiltering = true
				m.filterInput.Focus()
				return m, nil
			}
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
		case "ctrl+x":
			if m.focusIndex == 3 && len(m.jobs) > 0 {
				job := m.jobs[m.jobCursor]
				if job.Status == "running" && job.Cancel != nil {
					job.Cancel()
					m.jobs[m.jobCursor].Status = string(runner.StatusFailed)
					// uiJob doesn't have an Output string, use Result.Stderr
					m.jobs[m.jobCursor].Result.Stderr = "Job cancelled by user\n"
					m.statusMessage = "Cancelled job: " + job.Title
					m.updateViewportContent()
				}
			}
			return m, nil
		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
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
			if m.focusIndex == 2 {
				m.cycleArgs(-1)
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
			if m.focusIndex == 2 {
				m.cycleArgs(1)
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
	if m.showHelp {
		return m.renderHelp()
	}

	b := &strings.Builder{}
	b.WriteString(headerStyle.Render("Atlas-Recon Dashboard"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("tab: next  enter: select/run  esc: menu  ctrl+d: details  ?: help  ctrl+c: quit"))
	b.WriteString("\n\n")

	leftStr := &strings.Builder{}
	leftStr.WriteString(m.renderCommands())
	leftStr.WriteString("\n")
	leftStr.WriteString(m.renderInputs())
	leftStr.WriteString("\n")
	leftStr.WriteString(m.renderJobs())

	if m.width > 100 {
		// Side-by-side layout
		leftPaneWidth := m.width * 45 / 100
		leftPane := lipgloss.NewStyle().Width(leftPaneWidth).Render(leftStr.String())

		rightPane := ""
		if m.showDetails {
			if !m.ready {
				rightPane = borderStyle.Render("\n  Initializing...\n")
			} else {
				rightPane = borderStyle.Render(m.viewport.View())
			}
		}

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane))
	} else {
		// Stacked layout
		b.WriteString(leftStr.String())
		if m.showDetails {
			b.WriteString("\n")
			if !m.ready {
				b.WriteString("\n  Initializing...\n")
			} else {
				b.WriteString(m.viewport.View())
			}
		}
	}

	if m.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(m.statusMessage))
	}

	return b.String()
}

func (m model) renderHelp() string {
	b := &strings.Builder{}
	b.WriteString(headerStyle.Render("Atlas-Recon Help Session"))
	b.WriteString("\n\n")
	b.WriteString("Navigation:\n")
	b.WriteString("  tab / shift+tab   Move focus between panels (Menu -> Target -> Args -> Jobs)\n")
	b.WriteString("  up/down, j/k      Navigate selected panel (Commands, Args Options, Jobs)\n")
	b.WriteString("  /                 Filter commands (when in menu panel)\n")
	b.WriteString("  enter             Run selected command or focus next input\n")
	b.WriteString("  ctrl+d            Toggle output details panel\n")
	b.WriteString("  ?                 Toggle this help screen\n")
	b.WriteString("  q / ctrl+c        Quit application\n")
	b.WriteString("\n")
	b.WriteString("Job Management:\n")
	b.WriteString("  ctrl+x            Cancel selected running job\n")
	b.WriteString("\nPress ? or esc to return.")
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

func (m *model) filterCommands() {
	query := strings.ToLower(m.filterInput.Value())
	if query == "" {
		m.commands = m.allCommands
	} else {
		filtered := []commandDef{}
		for _, c := range m.allCommands {
			if strings.Contains(strings.ToLower(c.Name), query) || strings.Contains(strings.ToLower(c.Description), query) {
				filtered = append(filtered, c)
			}
		}
		m.commands = filtered
	}
	if m.menuIndex >= len(m.commands) {
		m.menuIndex = len(m.commands) - 1
		if m.menuIndex < 0 {
			m.menuIndex = 0
		}
	}
}

func (m model) renderCommands() string {
	b := &strings.Builder{}
	b.WriteString("Commands:\n")

	if m.isFiltering {
		b.WriteString(m.filterInput.View() + "\n")
	} else if m.focusIndex == 0 {
		b.WriteString(helpStyle.Render("press '/' to filter") + "\n")
	} else {
		b.WriteString("\n")
	}

	if len(m.commands) == 0 {
		b.WriteString("  (no commands match)\n")
		return b.String()
	}

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

	hint := ""
	if len(m.commands[m.menuIndex].ArgsOptions) > 0 {
		hint = " (↑/↓ to select category)"
	}
	b.WriteString(m.renderInput(m.argsInput, 2) + helpStyle.Render(hint))
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
		if job.Status == "running" {
			status = m.spinner.View() + " " + status
		}
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
		parsedArgs, err := shlex.Split(args)
		if err != nil {
			m.statusMessage = "invalid arguments"
			return m, nil
		}
		argList = append(argList, parsedArgs...)
	}

	id := fmt.Sprintf("dash-%d", time.Now().UnixNano())
	jobTitle := cmdDef.Name
	if target != "" {
		jobTitle += " " + target
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := uiJob{ID: id, Title: jobTitle, Status: "running", Cancel: cancel}

	script := pluginPath(cmdDef.Script)
	if err := m.queue.Submit(runner.Job{
		ID:      id,
		Command: cmdDef.Name,
		Args:    argList,
		Run: func(_ context.Context) (runner.Result, error) {
			result, err := runner.RunPython(script, argList, runner.RunOptions{
				Stream:  false,
				Python:  m.cfg.Paths.Python,
				Timeout: time.Duration(m.cfg.Timeouts.CommandSeconds) * time.Second,
				Context: ctx,
				APIKeys: m.cfg.APIKeys,
			})
			result.ID = id
			return result, err
		},
	}); err != nil {
		m.statusMessage = "job queue is stopped"
		return m, nil
	}

	m.jobs = append(m.jobs, job)
	m.jobCursor = len(m.jobs) - 1
	m.statusMessage = fmt.Sprintf("queued job: %s", jobTitle)

	return m, nil
}

func (m *model) cycleArgs(dir int) {
	cmd := m.commands[m.menuIndex]
	if len(cmd.ArgsOptions) == 0 {
		return
	}

	current := m.argsInput.Value()
	idx := -1

	// Find which option is currently selected
	for i, opt := range cmd.ArgsOptions {
		if strings.HasPrefix(current, opt) {
			idx = i
			break
		}
	}

	if idx == -1 {
		idx = 0
	} else {
		idx = (idx + dir) % len(cmd.ArgsOptions)
		if idx < 0 {
			idx = len(cmd.ArgsOptions) - 1
		}
	}

	// Extract any user arguments after the category
	var extraArgs string
	if len(current) > 0 {
		parts := strings.Fields(current)
		// Find parts that don't match any category option
		var extra []string
		for _, part := range parts {
			isCategory := false
			for _, opt := range cmd.ArgsOptions {
				if strings.HasPrefix(part, strings.Fields(opt)[0]) {
					isCategory = true
					break
				}
			}
			if !isCategory {
				extra = append(extra, part)
			}
		}
		if len(extra) > 0 {
			extraArgs = " " + strings.Join(extra, " ")
		}
	}

	val := cmd.ArgsOptions[idx] + extraArgs
	m.argsInput.SetValue(val)
	m.argsInput.SetCursor(len(val))
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
	if path, err := plugins.GetPluginPath(name); err == nil {
		return path
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
