package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type BranchStatus struct {
	Name      string
	IsCurrent bool
	Staged    bool
	Modified  bool
	Untracked bool
	Ahead     int
	Behind    int
}

type RepoStatus struct {
	Path      string
	Branch    string
	Staged    bool
	Modified  bool
	Untracked bool
	Ahead     int
	Behind    int
	Branches  []BranchStatus
	Error     string
}

type Config struct {
	Folders []string `json:"folders"`
}

type scanDoneMsg struct {
	statuses []RepoStatus
	err      error
}

type lazygitDoneMsg struct {
	err error
}

type nvimDoneMsg struct {
	err error
}

type shellDoneMsg struct {
	err error
}

type yaziDoneMsg struct {
	err error
}

type model struct {
	configPath string
	statuses   []RepoStatus
	selected   int
	loading    bool
	err        error
	notice     string
	width      int
	height     int
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	selectedStyle = lipgloss.NewStyle().Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	cleanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
)

func main() {
	defaultConfig := defaultConfigPath()

	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(defaultConfig); err != nil {
			fmt.Fprintf(os.Stderr, "init error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created config at %s\n", defaultConfig)
		return
	}

	configPath := flag.String("config", defaultConfig, "Path to config file")
	flag.Parse()

	m := model{configPath: *configPath, loading: true}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	return scanCmd(m.configPath)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case scanDoneMsg:
		m.loading = false
		m.statuses = msg.statuses
		m.err = msg.err
		if m.selected >= len(m.statuses) {
			m.selected = max(0, len(m.statuses)-1)
		}
		return m, nil
	case lazygitDoneMsg:
		if msg.err != nil {
			m.notice = "lazygit failed: " + msg.err.Error()
		} else {
			m.notice = ""
		}
		return m, nil
	case nvimDoneMsg:
		if msg.err != nil {
			m.notice = "nvim failed: " + msg.err.Error()
		} else {
			m.notice = ""
		}
		return m, nil
	case shellDoneMsg:
		if msg.err != nil {
			m.notice = "shell failed: " + msg.err.Error()
		} else {
			m.notice = ""
		}
		return m, nil
	case yaziDoneMsg:
		if msg.err != nil {
			m.notice = "yazi failed: " + msg.err.Error()
		} else {
			m.notice = ""
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = nil
			m.notice = ""
			return m, scanCmd(m.configPath)
		case "g":
			if len(m.statuses) == 0 {
				return m, nil
			}
			repoPath := m.statuses[m.selected].Path
			if _, err := exec.LookPath("lazygit"); err != nil {
				m.notice = "lazygit not installed"
				return m, nil
			}
			m.notice = ""
			return m, openLazygitCmd(repoPath)
		case "v":
			if len(m.statuses) == 0 {
				return m, nil
			}
			repoPath := m.statuses[m.selected].Path
			if _, err := exec.LookPath("nvim"); err != nil {
				m.notice = "nvim not installed"
				return m, nil
			}
			m.notice = ""
			return m, openNvimCmd(repoPath)
		case "o":
			if len(m.statuses) == 0 {
				return m, nil
			}
			repoPath := m.statuses[m.selected].Path
			m.notice = ""
			return m, openShellCmd(repoPath)
		case "y":
			if len(m.statuses) == 0 {
				return m, nil
			}
			repoPath := m.statuses[m.selected].Path
			if _, err := exec.LookPath("yazi"); err != nil {
				m.notice = "yazi not installed"
				return m, nil
			}
			m.notice = ""
			return m, openYaziCmd(repoPath)
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.statuses)-1 {
				m.selected++
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.loading {
		return "\n  Scanning repositories..."
	}
	if m.err != nil {
		return "\n  " + errorStyle.Render("Error: "+m.err.Error()) + "\n\n  Press r to retry, q to quit."
	}

	help := "↑/↓ move • g lazygit • v nvim • y yazi • o shell • r rescan • q quit"
	if m.notice != "" {
		help += " • " + m.notice
	}
	header := titleStyle.Render("GopherHole Repo Status") + "\n" + dimStyle.Render(help) + "\n\n"
	if len(m.statuses) == 0 {
		return header + warnStyle.Render("No Git repositories found.")
	}

	left := panelStyle.Render(m.renderList())
	right := panelStyle.Render(m.renderDetail())
	return header + lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right)
}

func (m model) renderList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Repositories") + "\n")

	const (
		nameW   = 28
		stateW  = 22
		countW  = 8
		dirtyW  = 6
		untrkW  = 10
	)

	lastGroup := -1
	for i, s := range m.statuses {
		group := statusGroupRank(s)
		if group != lastGroup {
			if lastGroup != -1 {
				b.WriteString("\n")
			}
			b.WriteString(dimStyle.Render(statusGroupLabel(group)) + "\n")
			header := fmt.Sprintf("  %-*s %-*s %*s %*s %*s", nameW, "REPO", stateW, "STATE", countW, "BRANCHES", dirtyW, "DIRTY", untrkW, "UNTRACKED")
			b.WriteString(dimStyle.Render(header) + "\n")
			lastGroup = group
		}

		name := truncateRight(filepath.Base(s.Path), nameW)
		state := stateText(s)
		dirtyCount, untrackedCount := branchCounts(s)
		line := fmt.Sprintf("  %-*s %-*s %*d %*d %*d", nameW, name, stateW, state, countW, len(s.Branches), dirtyW, dirtyCount, untrkW, untrackedCount)
		if i == m.selected {
			line = selectedStyle.Render("›" + line[1:])
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m model) renderDetail() string {
	s := m.statuses[m.selected]
	statusStyled := styleForState(s).Render(stateText(s))

	lines := []string{
		titleStyle.Render("Details"),
		"Path:    " + s.Path,
		"Branch:  " + s.Branch,
		fmt.Sprintf("Sync:    ahead %d / behind %d", s.Ahead, s.Behind),
		"Status:  " + statusStyled,
		"",
		titleStyle.Render("Branches"),
		"NAME                     STATUS       AHEAD  BEHIND",
	}

	for _, br := range s.Branches {
		name := br.Name
		if br.IsCurrent {
			name = "* " + name
		} else {
			name = "  " + name
		}
		bs := branchStateText(br)
		lines = append(lines, fmt.Sprintf("%-24s %-12s %5d %7d", name, bs, br.Ahead, br.Behind))
	}

	if s.Error != "" {
		lines = append(lines, "", "Error:   "+errorStyle.Render(s.Error))
	}
	return strings.Join(lines, "\n")
}

func openLazygitCmd(repoPath string) tea.Cmd {
	cmd := exec.Command("lazygit")
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return lazygitDoneMsg{err: err}
	})
}

func openNvimCmd(repoPath string) tea.Cmd {
	cmd := exec.Command("nvim")
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return nvimDoneMsg{err: err}
	})
}

func openShellCmd(repoPath string) tea.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.Command(shell)
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return shellDoneMsg{err: err}
	})
}

func openYaziCmd(repoPath string) tea.Cmd {
	cmd := exec.Command("yazi")
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return yaziDoneMsg{err: err}
	})
}

func scanCmd(configPath string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := loadConfig(configPath)
		if err != nil {
			return scanDoneMsg{err: err}
		}
		if len(cfg.Folders) == 0 {
			return scanDoneMsg{err: errors.New("no folders configured")}
		}

		repos, err := findGitRepos(cfg.Folders)
		if err != nil {
			return scanDoneMsg{err: err}
		}

		statuses := make([]RepoStatus, 0, len(repos))
		for _, repo := range repos {
			statuses = append(statuses, getRepoStatus(repo))
		}
		sort.Slice(statuses, func(i, j int) bool {
			gi := statusGroupRank(statuses[i])
			gj := statusGroupRank(statuses[j])
			if gi != gj {
				return gi < gj
			}
			return strings.ToLower(statuses[i].Path) < strings.ToLower(statuses[j].Path)
		})
		return scanDoneMsg{statuses: statuses}
	}
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, fmt.Errorf("config not found at %s", path)
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "repos.config.json"
	}
	return filepath.Join(home, ".config", "gopherhole", "repos.config.json")
}

func runInit(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists at %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	template := "{\n  \"folders\": [\n    \"/path/to/work\",\n    \"/path/to/personal\"\n  ]\n}\n"
	return os.WriteFile(path, []byte(template), 0o644)
}

func findGitRepos(roots []string) ([]string, error) {
	seen := make(map[string]struct{})
	var repos []string

	for _, root := range roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			name := d.Name()
			if name == "node_modules" {
				return filepath.SkipDir
			}
			if name == ".git" {
				repoPath := filepath.Dir(path)
				if _, ok := seen[repoPath]; !ok {
					seen[repoPath] = struct{}{}
					repos = append(repos, repoPath)
				}
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return repos, nil
}

func getRepoStatus(repoPath string) RepoStatus {
	s := RepoStatus{Path: repoPath}

	branch, err := gitOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		s.Error = err.Error()
		return s
	}
	s.Branch = strings.TrimSpace(branch)

	ab, err := gitOutput(repoPath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err == nil {
		fields := strings.Fields(ab)
		if len(fields) == 2 {
			fmt.Sscanf(fields[0], "%d", &s.Ahead)
			fmt.Sscanf(fields[1], "%d", &s.Behind)
		}
	}

	porcelain, err := gitOutput(repoPath, "status", "--porcelain")
	if err != nil {
		s.Error = err.Error()
		return s
	}
	for _, line := range strings.Split(strings.TrimSpace(porcelain), "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' && y == '?' {
			s.Untracked = true
			continue
		}
		if x != ' ' {
			s.Staged = true
		}
		if y != ' ' {
			s.Modified = true
		}
	}

	branches, berr := listBranches(repoPath, s.Branch, s.Staged, s.Modified, s.Untracked)
	if berr == nil {
		s.Branches = branches
	}
	return s
}

func gitOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", err
		}
		return "", errors.New(msg)
	}
	return string(out), nil
}

func listBranches(repoPath, current string, currentStaged, currentModified, currentUntracked bool) ([]BranchStatus, error) {
	out, err := gitOutput(repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}

	var branches []BranchStatus
	for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(name) == "" {
			continue
		}
		b := BranchStatus{Name: strings.TrimSpace(name), IsCurrent: strings.TrimSpace(name) == current}
		ab, abErr := gitOutput(repoPath, "rev-list", "--left-right", "--count", b.Name+"..."+b.Name+"@{upstream}")
		if abErr == nil {
			fields := strings.Fields(ab)
			if len(fields) == 2 {
				fmt.Sscanf(fields[0], "%d", &b.Ahead)
				fmt.Sscanf(fields[1], "%d", &b.Behind)
			}
		}
		if b.IsCurrent {
			b.Staged = currentStaged
			b.Modified = currentModified
			b.Untracked = currentUntracked
		}
		branches = append(branches, b)
	}

	sort.Slice(branches, func(i, j int) bool {
		if branches[i].IsCurrent != branches[j].IsCurrent {
			return branches[i].IsCurrent
		}
		return strings.ToLower(branches[i].Name) < strings.ToLower(branches[j].Name)
	})
	return branches, nil
}

func stateText(s RepoStatus) string {
	if s.Error != "" {
		return "error"
	}
	parts := make([]string, 0, 3)
	if s.Modified || s.Staged {
		parts = append(parts, "changes")
	}
	if s.Untracked {
		parts = append(parts, "untracked")
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, "+")
}

func styleForState(s RepoStatus) lipgloss.Style {
	if s.Error != "" {
		return errorStyle
	}
	if s.Modified || s.Staged {
		return dirtyStyle
	}
	if s.Untracked {
		return warnStyle
	}
	return cleanStyle
}

func statusGroupRank(s RepoStatus) int {
	if s.Error != "" {
		return 3
	}
	if s.Modified || s.Staged {
		return 0
	}
	if s.Untracked {
		return 1
	}
	return 2
}

func truncateRight(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

func branchStateText(b BranchStatus) string {
	if b.Modified || b.Staged {
		return dirtyStyle.Render("changes")
	}
	if b.Untracked {
		return warnStyle.Render("untracked")
	}
	return cleanStyle.Render("clean")
}

func branchCounts(s RepoStatus) (dirty int, untracked int) {
	for _, b := range s.Branches {
		if b.Modified || b.Staged {
			dirty++
		}
		if b.Untracked {
			untracked++
		}
	}
	return dirty, untracked
}

func statusGroupLabel(rank int) string {
	switch rank {
	case 0:
		return "Changes"
	case 1:
		return "Untracked"
	case 2:
		return "Clean"
	default:
		return "Errors"
	}
}
