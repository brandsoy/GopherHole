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

type RepoStatus struct {
	Path      string
	Branch    string
	Staged    bool
	Modified  bool
	Untracked bool
	Ahead     int
	Behind    int
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
)

func main() {
	configPath := flag.String("config", "repos.config.json", "Path to config file")
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

	help := "↑/↓ move • g open lazygit • r rescan • q quit"
	if m.notice != "" {
		help += " • " + m.notice
	}
	header := titleStyle.Render("GopherHole Repo Status") + "\n" + dimStyle.Render(help) + "\n\n"
	if len(m.statuses) == 0 {
		return header + warnStyle.Render("No Git repositories found.")
	}

	left := m.renderList()
	right := m.renderDetail()
	return header + lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m model) renderList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Repositories") + "\n")

	lastGroup := -1
	for i, s := range m.statuses {
		group := statusGroupRank(s)
		if group != lastGroup {
			if lastGroup != -1 {
				b.WriteString("\n")
			}
			b.WriteString(dimStyle.Render(statusGroupLabel(group)) + "\n")
			lastGroup = group
		}

		state := stateText(s)
		line := fmt.Sprintf("%s  %s", filepath.Base(s.Path), state)
		if i == m.selected {
			line = selectedStyle.Render("› " + line)
		} else {
			line = "  " + line
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
	}
	if s.Error != "" {
		lines = append(lines, "Error:   "+errorStyle.Render(s.Error))
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
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
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
