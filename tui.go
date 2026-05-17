package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) Init() tea.Cmd {
	return tea.Batch(scanCmd(m.configPath), spinTickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		if !m.busy {
			return m, nil
		}
		m.spinIndex = (m.spinIndex + 1) % len(spinnerFrames)
		return m, spinTickCmd()
	case scanDoneMsg:
		m.loading, m.busy, m.busyLabel, m.busyStartedAt = false, false, "", time.Time{}
		m.statuses, m.err = msg.statuses, msg.err
		if m.selected >= len(m.statuses) {
			m.selected = max(0, len(m.statuses)-1)
		}
		return m, nil
	case lazygitDoneMsg:
		m.notice = doneNotice("lazygit", msg.err)
		return m, nil
	case nvimDoneMsg:
		m.notice = doneNotice("nvim", msg.err)
		return m, nil
	case shellDoneMsg:
		m.notice = doneNotice("shell", msg.err)
		return m, nil
	case yaziDoneMsg:
		m.notice = doneNotice("yazi", msg.err)
		return m, nil
	case reportOpenDoneMsg:
		if msg.err != nil {
			m.notice = "report open failed: " + msg.err.Error()
		} else {
			m.notice = ""
		}
		return m, nil
	case leakTodoDoneMsg:
		if msg.err != nil {
			m.notice = "todo creation failed: " + msg.err.Error()
		} else {
			m.notice = "created " + msg.path
		}
		return m, nil
	case gitleaksDoneMsg:
		m.busy, m.busyLabel, m.busyStartedAt = false, "", time.Time{}
		if msg.err != nil {
			m.notice = "gitleaks " + msg.mode + " failed"
		} else if msg.ok {
			m.notice = "gitleaks " + msg.mode + " clean"
		} else {
			m.notice = fmt.Sprintf("gitleaks %s found %d leaks", msg.mode, msg.leakCount)
		}
		for i := range m.statuses {
			if m.statuses[i].Path != msg.repoPath {
				continue
			}
			if msg.mode == "local" {
				m.statuses[i].LocalLeakScanned = true
				m.statuses[i].LocalLeakCount = msg.leakCount
				m.statuses[i].LatestLocalReport = msg.reportPath
			} else {
				m.statuses[i].RemoteLeakScanned = true
				m.statuses[i].RemoteLeakCount = msg.leakCount
				m.statuses[i].LatestRemoteReport = msg.reportPath
			}
			m.statuses[i].LatestAnyReport = msg.reportPath
			break
		}
		m.gitleaksReport = formatGitleaksReport(msg)
		m.gitleaksReportPath = msg.reportPath
		m.showGitleaksPopup = true
		return m, nil
	case bulkRepoDoneMsg:
		res := msg.result
		for i := range m.statuses {
			if m.statuses[i].Path == res.repoPath {
				m.statuses[i].LocalLeakScanned = true
				m.statuses[i].LocalLeakCount = res.leakCount
				m.statuses[i].LatestLocalReport = res.reportPath
				m.statuses[i].LatestAnyReport = res.reportPath
				break
			}
		}
		if m.bulkIndex < len(m.bulkItems) {
			m.bulkItems[m.bulkIndex].Leaks = res.leakCount
			if res.err != nil {
				m.bulkItems[m.bulkIndex].State = "failed"
			} else {
				m.bulkItems[m.bulkIndex].State = "done"
			}
		}
		m.bulkLeakTotal += res.leakCount
		m.bulkIndex++
		if m.bulkIndex >= len(m.bulkRepoPaths) {
			m.busy, m.busyLabel, m.busyStartedAt = false, "", time.Time{}
			m.notice = fmt.Sprintf("bulk gitleaks done: %d repos, %d total leaks", len(m.bulkRepoPaths), m.bulkLeakTotal)
			return m, nil
		}
		next := m.bulkRepoPaths[m.bulkIndex]
		if m.bulkIndex < len(m.bulkItems) {
			m.bulkItems[m.bulkIndex].State = "running"
		}
		m.busyLabel = fmt.Sprintf("Gitleaks local %d/%d: %s", m.bulkIndex+1, len(m.bulkRepoPaths), filepath.Base(next))
		return m, runBulkLocalRepoCmd(next)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showBulkConfirm {
		switch msg.String() {
		case keyConfirmYes:
			if len(m.bulkRepoPaths) == 0 {
				m.showBulkConfirm = false
				m.notice = "no repositories to scan"
				return m, nil
			}
			m.showBulkConfirm = false
			m.bulkItems = make([]bulkScanItem, len(m.bulkRepoPaths))
			for i, p := range m.bulkRepoPaths {
				m.bulkItems[i] = bulkScanItem{RepoPath: p, State: "pending"}
			}
			m.bulkItems[0].State = "running"
			m.bulkIndex = 0
			m.bulkLeakTotal = 0
			m.busy, m.busyStartedAt = true, time.Now()
			m.busyLabel = fmt.Sprintf("Gitleaks local %d/%d: %s", 1, len(m.bulkRepoPaths), filepath.Base(m.bulkRepoPaths[0]))
			m.notice = ""
			return m, runBulkLocalRepoCmd(m.bulkRepoPaths[0])
		case keyConfirmNo, keyEsc, keyClear:
			m.showBulkConfirm = false
			m.notice = ""
			return m, nil
		default:
			return m, nil
		}
	}

	if m.showGitleaksPopup {
		switch msg.String() {
		case keyEsc, keyClear:
			m.showGitleaksPopup = false
			return m, nil
		case keyPopupOpen, keyEnter:
			if m.gitleaksReportPath == "" {
				m.notice = "no gitleaks report yet"
				return m, nil
			}
			if _, err := os.Stat(m.gitleaksReportPath); err != nil {
				m.notice = "report file missing"
				return m, nil
			}
			return m, openReportCmd(m.gitleaksReportPath)
		case keyTodoFile:
			if m.gitleaksReportPath == "" {
				m.notice = "no gitleaks report yet"
				return m, nil
			}
			if len(m.statuses) == 0 {
				m.notice = "no repository selected"
				return m, nil
			}
			repoPath := m.statuses[m.selected].Path
			return m, createLeaksTodoCmd(m.gitleaksReportPath, repoPath)
		}
	}

	if m.showGitMenu {
		switch msg.String() {
		case keyEsc, keyClear:
			m.showGitMenu = false
			return m, nil
		case keyUp, keyUpAlt:
			if m.gitMenuIndex > 0 {
				m.gitMenuIndex--
			}
			return m, nil
		case keyDown, keyDownAlt:
			if m.gitMenuIndex < 2 {
				m.gitMenuIndex++
			}
			return m, nil
		case keyEnter:
			return m.runGitMenuAction()
		case keyGitMenuLazygit:
			m.gitMenuIndex = 0
			return m.runGitMenuAction()
		case keyGitMenuLeakL:
			m.gitMenuIndex = 1
			return m.runGitMenuAction()
		case keyGitMenuLeakR:
			m.gitMenuIndex = 2
			return m.runGitMenuAction()
		}
		return m, nil
	}

	switch msg.String() {
	case keyQuit, keyQuitAlt:
		return m, tea.Quit
	case keyRefresh:
		m.loading, m.busy, m.busyLabel, m.busyStartedAt = true, true, "Refreshing repositories", time.Now()
		m.err, m.notice = nil, ""
		m.bulkItems = nil
		return m, tea.Batch(scanCmd(m.configPath), spinTickCmd())
	case keyClear:
		m.gitleaksReport, m.gitleaksReportPath, m.notice = "", "", ""
		m.showGitleaksPopup = false
		return m, nil
	case keyUp, keyUpAlt:
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case keyDown, keyDownAlt:
		if m.selected < len(m.statuses)-1 {
			m.selected++
		}
		return m, nil
	case keyPopupOpen:
		if len(m.statuses) == 0 {
			m.notice = "no repositories"
			return m, nil
		}
		reportPath := m.statuses[m.selected].LatestAnyReport
		if reportPath == "" {
			reportPath = m.gitleaksReportPath
		}
		if reportPath == "" {
			m.notice = "no gitleaks report yet"
			return m, nil
		}
		if _, err := os.Stat(reportPath); err != nil {
			m.notice = "report file missing"
			return m, nil
		}
		return m, openReportCmd(reportPath)
	case keyBulkLeakAll:
		if len(m.statuses) == 0 {
			m.notice = "no repositories to scan"
			return m, nil
		}
		m.showBulkConfirm = true
		m.bulkRepoPaths = make([]string, len(m.statuses))
		for i, s := range m.statuses {
			m.bulkRepoPaths[i] = s.Path
		}
		m.bulkItems = nil
		m.notice = ""
		return m, nil
	case keyConfigNvim:
		if _, err := requireTool("nvim"); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.notice = ""
		return m, openNvimFileCmd(m.configPath)
	}

	if len(m.statuses) == 0 {
		return m, nil
	}
	repoPath := m.statuses[m.selected].Path

	switch msg.String() {
	case keyGitMenu:
		m.showGitMenu = true
		m.gitMenuIndex = 0
		m.notice = ""
		return m, nil
	case keyNvim:
		if _, err := requireTool("nvim"); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.notice = ""
		return m, openNvimCmd(repoPath)
	case keyYazi:
		if _, err := requireTool("yazi"); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.notice = ""
		return m, openYaziCmd(repoPath)
	case keyShell:
		m.notice = ""
		return m, openShellCmd(repoPath)
	}

	return m, nil
}

func (m model) View() string {
	if m.loading {
		return "\n  " + spinnerFrames[m.spinIndex] + " " + m.busyLabel + "..." + busyElapsedText(m.busyStartedAt)
	}
	if m.err != nil {
		return "\n  " + errorStyle.Render("Error: "+m.err.Error()) + "\n\n  Press r to retry, q to quit."
	}

	help := helpText
	if m.busy {
		help += " • " + spinnerFrames[m.spinIndex] + " " + m.busyLabel + busyElapsedText(m.busyStartedAt)
	}
	if m.notice != "" {
		help += " • " + m.notice
	}
	header := titleStyle.Render("GopherHole Repo Status") + "\n\n"
	var body string
	if len(m.statuses) == 0 {
		body = warnStyle.Render("No Git repositories found.")
	} else {
		left := panelStyle.Render(m.renderList())
		rightTop := panelStyle.Width(detailPanelWidth).Render(m.renderDetail())
		right := rightTop
		if len(m.bulkItems) > 0 {
			rightBottom := panelStyle.Width(detailPanelWidth).Render(m.renderBulkProgress())
			right = lipgloss.JoinVertical(lipgloss.Left, rightTop, "", rightBottom)
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right)
	}

	base := header + body
	footer := dimStyle.Render(help)
	if m.height > 0 {
		used := lipgloss.Height(base) + 1
		if m.showGitleaksPopup && m.gitleaksReport != "" {
			popup := panelStyle.Width(popupPanelWidth).Render(m.gitleaksReport + "\n\n" + dimStyle.Render("p/enter: open detailed report • t: create LEAKS_TODO.md • c/esc: close"))
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
		}
		if used < m.height {
			base += strings.Repeat("\n", m.height-used)
		}
	}

	if m.showGitleaksPopup && m.gitleaksReport != "" {
		popup := panelStyle.Width(popupPanelWidth).Render(m.gitleaksReport + "\n\n" + dimStyle.Render("p/enter: open detailed report • t: create LEAKS_TODO.md • c/esc: close"))
		if m.width > 0 && m.height > 0 {
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
		}
		return "\n\n" + popup
	}

	if m.showBulkConfirm {
		msg := strings.Join([]string{
			titleStyle.Render("Confirm bulk scan"),
			fmt.Sprintf("Run gitleaks local on %d repositories?", len(m.bulkRepoPaths)),
			"",
			dimStyle.Render("y: yes • n/esc/c: cancel"),
		}, "\n")
		popup := panelStyle.Width(56).Render(msg)
		if m.width > 0 && m.height > 0 {
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
		}
		return "\n\n" + popup
	}

	if m.showGitMenu {
		repoPath := ""
		if len(m.statuses) > 0 {
			repoPath = m.statuses[m.selected].Path
		}
		items := []string{
			"g) Open lazygit",
			"l) Run gitleaks local",
			"L) Run gitleaks remote",
		}
		for i := range items {
			prefix := "  "
			if i == m.gitMenuIndex {
				prefix = "› "
			}
			items[i] = prefix + items[i]
		}
		menu := strings.Join([]string{
			titleStyle.Render("Git Menu"),
			dimStyle.Render("Repo:"),
			truncateRight(repoPath, 58),
			"",
			strings.Join(items, "\n"),
			"",
			dimStyle.Render("↑/↓ select • enter run • g/l/L hotkeys • esc/c close"),
		}, "\n")
		popup := panelStyle.Width(64).Render(menu)
		if m.width > 0 && m.height > 0 {
			return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
		}
		return "\n\n" + popup
	}

	return base + "\n" + footer
}

func (m model) renderList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Repositories") + "\n")

	const nameW, stateW, countW, dirtyW, untrkW, leakLW, leakRW = 24, 22, 8, 6, 10, 8, 8
	lastGroup := -1
	for i, s := range m.statuses {
		group := statusGroupRank(s)
		if group != lastGroup {
			if lastGroup != -1 {
				b.WriteString("\n")
			}
			b.WriteString(dimStyle.Render(statusGroupLabel(group)) + "\n")
			h := fmt.Sprintf("  %-*s %-*s %*s %*s %*s %*s %*s", nameW, "REPO", stateW, "STATE", countW, "BRANCHES", dirtyW, "DIRTY", untrkW, "UNTRACKED", leakLW, "L-LEAKS", leakRW, "R-LEAKS")
			b.WriteString(dimStyle.Render(h) + "\n")
			lastGroup = group
		}

		localLeaks, remoteLeaks := "-", "-"
		if s.LocalLeakScanned {
			localLeaks = fmt.Sprintf("%d", s.LocalLeakCount)
		}
		if s.RemoteLeakScanned {
			remoteLeaks = fmt.Sprintf("%d", s.RemoteLeakCount)
		}
		dirtyCount, untrackedCount := branchCounts(s)
		line := fmt.Sprintf("  %-*s %-*s %*d %*d %*d %*s %*s", nameW, truncateRight(filepath.Base(s.Path), nameW), stateW, stateText(s), countW, len(s.Branches), dirtyW, dirtyCount, untrkW, untrackedCount, leakLW, localLeaks, leakRW, remoteLeaks)
		if i == m.selected {
			line = selectedStyle.Render("›" + line[1:])
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m model) renderDetail() string {
	s := m.statuses[m.selected]
	lines := []string{
		titleStyle.Render("Details"),
		"Path:    " + s.Path,
		"Branch:  " + s.Branch,
		fmt.Sprintf("Sync:    ahead %d / behind %d", s.Ahead, s.Behind),
		"Status:  " + styleForState(s).Render(stateText(s)),
		"",
		titleStyle.Render("Branches"),
		"NAME                     STATUS       AHEAD  BEHIND",
	}
	for _, br := range s.Branches {
		name := "  " + br.Name
		if br.IsCurrent {
			name = "* " + br.Name
		}
		lines = append(lines, fmt.Sprintf("%-24s %-12s %5d %7d", name, branchStateText(br), br.Ahead, br.Behind))
	}
	if s.Error != "" {
		lines = append(lines, "", "Error:   "+errorStyle.Render(s.Error))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderBulkProgress() string {
	lines := []string{titleStyle.Render("Bulk local gitleaks progress")}
	done := 0
	for _, it := range m.bulkItems {
		if it.State == "done" || it.State == "failed" {
			done++
		}
	}
	lines = append(lines, fmt.Sprintf("Progress: %d/%d", done, len(m.bulkItems)))
	maxRows := 10
	if len(m.bulkItems) < maxRows {
		maxRows = len(m.bulkItems)
	}
	for i := 0; i < maxRows; i++ {
		it := m.bulkItems[i]
		marker := "…"
		state := "pending"
		switch it.State {
		case "running":
			marker = spinnerFrames[m.spinIndex]
			state = "running"
		case "done":
			marker = "✓"
			state = fmt.Sprintf("done (%d leaks)", it.Leaks)
		case "failed":
			marker = "✗"
			state = "failed"
		}
		lines = append(lines, fmt.Sprintf("%s %-18s %s", marker, truncateRight(filepath.Base(it.RepoPath), 18), state))
	}
	if len(m.bulkItems) > maxRows {
		lines = append(lines, fmt.Sprintf("... and %d more", len(m.bulkItems)-maxRows))
	}
	return strings.Join(lines, "\n")
}

func (m model) runGitMenuAction() (tea.Model, tea.Cmd) {
	if len(m.statuses) == 0 {
		return m, nil
	}
	repoPath := m.statuses[m.selected].Path
	switch m.gitMenuIndex {
	case 0:
		if _, err := requireTool("lazygit"); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.showGitMenu = false
		m.notice = ""
		return m, openLazygitCmd(repoPath)
	case 1:
		if _, err := requireTool("gitleaks"); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.showGitMenu = false
		m.notice = ""
		m.busy, m.busyLabel, m.busyStartedAt = true, "Running gitleaks local", time.Now()
		return m, tea.Batch(runGitleaksLocalCmd(repoPath), spinTickCmd())
	case 2:
		if _, err := requireTool("gitleaks"); err != nil {
			m.notice = err.Error()
			return m, nil
		}
		m.showGitMenu = false
		m.notice = ""
		m.busy, m.busyLabel, m.busyStartedAt = true, "Running gitleaks remote", time.Now()
		return m, tea.Batch(runGitleaksRemoteCmd(repoPath), spinTickCmd())
	}
	return m, nil
}

func doneNotice(name string, err error) string {
	if err != nil {
		return name + " failed: " + err.Error()
	}
	return ""
}

func requireTool(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not installed", name)
	}
	return path, nil
}
