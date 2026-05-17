package main

import (
	"time"

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
	Path               string
	Branch             string
	Staged             bool
	Modified           bool
	Untracked          bool
	Ahead              int
	Behind             int
	Branches           []BranchStatus
	LocalLeakCount     int
	RemoteLeakCount    int
	LocalLeakScanned   bool
	RemoteLeakScanned  bool
	LatestLocalReport  string
	LatestRemoteReport string
	LatestAnyReport    string
	Error              string
}

type Config struct {
	Folders []string `json:"folders"`
}

type scanDoneMsg struct {
	statuses []RepoStatus
	err      error
}

type lazygitDoneMsg struct{ err error }
type nvimDoneMsg struct{ err error }
type shellDoneMsg struct{ err error }
type yaziDoneMsg struct{ err error }
type reportOpenDoneMsg struct{ err error }
type leakTodoDoneMsg struct {
	path string
	err  error
}
type tickMsg struct{}

type gitleaksDoneMsg struct {
	repoPath   string
	mode       string
	ok         bool
	output     string
	reportPath string
	leakCount  int
	err        error
}

type bulkRepoDoneMsg struct {
	result gitleaksDoneMsg
}

type bulkScanItem struct {
	RepoPath string
	State    string // pending, running, done, failed
	Leaks    int
}

type model struct {
	configPath         string
	statuses           []RepoStatus
	selected           int
	loading            bool
	busy               bool
	busyLabel          string
	busyStartedAt      time.Time
	spinIndex          int
	err                error
	notice             string
	gitleaksReport     string
	gitleaksReportPath string
	showGitleaksPopup  bool
	showGitMenu        bool
	showBulkConfirm    bool
	gitMenuIndex       int
	bulkRepoPaths      []string
	bulkItems          []bulkScanItem
	bulkIndex          int
	bulkLeakTotal      int
	width              int
	height             int
}

var (
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	titleStyle    = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	cleanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	panelStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
)
