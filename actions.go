package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func openLazygitCmd(repoPath string) tea.Cmd {
	cmd := exec.Command("lazygit")
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return lazygitDoneMsg{err: err} })
}

func openNvimCmd(repoPath string) tea.Cmd {
	cmd := exec.Command("nvim")
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return nvimDoneMsg{err: err} })
}

func openNvimFileCmd(path string) tea.Cmd {
	cmd := exec.Command("nvim", path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return nvimDoneMsg{err: err} })
}

func openShellCmd(repoPath string) tea.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.Command(shell)
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return shellDoneMsg{err: err} })
}

func openYaziCmd(repoPath string) tea.Cmd {
	cmd := exec.Command("yazi")
	cmd.Dir = repoPath
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return yaziDoneMsg{err: err} })
}

func busyElapsedText(start time.Time) string {
	if start.IsZero() {
		return ""
	}
	return fmt.Sprintf(" (%ds)", int(time.Since(start).Seconds()))
}

func spinTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func runGitleaksLocalCmd(repoPath string) tea.Cmd {
	return runGitleaksCmd("local", repoPath, "detect", "--source", ".", "--redact")
}

func runBulkLocalRepoCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		return bulkRepoDoneMsg{result: runGitleaksNow("local", repoPath, "detect", "--source", ".", "--redact")}
	}
}

func runGitleaksRemoteCmd(repoPath string) tea.Cmd {
	return runGitleaksCmd("remote", repoPath, "git", "--redact")
}

func runGitleaksCmd(mode, repoPath string, args ...string) tea.Cmd {
	return func() tea.Msg {
		return runGitleaksNow(mode, repoPath, args...)
	}
}

func runGitleaksNow(mode, repoPath string, args ...string) gitleaksDoneMsg {
	if err := ensureReportsDir(); err != nil {
		return gitleaksDoneMsg{mode: mode, err: err}
	}
	reportPath := reportFilePath(repoPath, mode)

	fullArgs := append(args, "--report-format", "json", "--report-path", reportPath)
	cmd := exec.Command("gitleaks", fullArgs...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	leakCount := countGitleaksFindings(reportPath)

	if err == nil {
		appendReportIndex(reportIndexEntry{CreatedAt: time.Now().Format(time.RFC3339), RepoPath: repoPath, Mode: mode, Leaks: leakCount, Path: reportPath})
		return gitleaksDoneMsg{repoPath: repoPath, mode: mode, ok: true, output: output, reportPath: reportPath, leakCount: leakCount}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		appendReportIndex(reportIndexEntry{CreatedAt: time.Now().Format(time.RFC3339), RepoPath: repoPath, Mode: mode, Leaks: leakCount, Path: reportPath})
		return gitleaksDoneMsg{repoPath: repoPath, mode: mode, ok: false, output: output, reportPath: reportPath, leakCount: leakCount}
	}
	return gitleaksDoneMsg{repoPath: repoPath, mode: mode, err: err, output: output, reportPath: reportPath, leakCount: leakCount}
}

func openReportCmd(path string) tea.Cmd {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}
	if _, err := exec.LookPath(pager); err == nil {
		cmd := exec.Command(pager, path)
		return tea.ExecProcess(cmd, func(err error) tea.Msg { return reportOpenDoneMsg{err: err} })
	}
	if _, err := exec.LookPath("nvim"); err == nil {
		cmd := exec.Command("nvim", path)
		return tea.ExecProcess(cmd, func(err error) tea.Msg { return reportOpenDoneMsg{err: err} })
	}
	return func() tea.Msg { return reportOpenDoneMsg{err: errors.New("no pager or nvim found")} }
}

func createLeaksTodoCmd(reportPath, repoPath string) tea.Cmd {
	return func() tea.Msg {
		todoPath, err := createLeaksTodo(reportPath, repoPath)
		return leakTodoDoneMsg{path: todoPath, err: err}
	}
}

func createLeaksTodo(reportPath, repoPath string) (string, error) {
	b, err := os.ReadFile(reportPath)
	if err != nil {
		return "", err
	}
	var findings []map[string]any
	if err := json.Unmarshal(b, &findings); err != nil {
		return "", err
	}

	var lines []string
	lines = append(lines, "# Gitleaks TODO")
	lines = append(lines, "")
	lines = append(lines, "Report: `"+reportPath+"`")
	lines = append(lines, "")
	if len(findings) == 0 {
		lines = append(lines, "No leaks found ✅")
	} else {
		for i, f := range findings {
			file := anyToString(f["File"])
			if file == "" {
				file = anyToString(f["file"])
			}
			if repoPath != "" {
				if rel, err := filepath.Rel(repoPath, file); err == nil && !strings.HasPrefix(rel, "..") {
					file = rel
				}
			}
			rule := anyToString(f["RuleID"])
			if rule == "" {
				rule = anyToString(f["ruleID"])
			}
			desc := anyToString(f["Description"])
			startLine := anyToInt(f["StartLine"])
			secret := anyToString(f["Secret"])
			if secret == "" {
				secret = anyToString(f["Match"])
			}
			if len(secret) > 120 {
				secret = secret[:120] + "..."
			}
			lines = append(lines, fmt.Sprintf("## [%d] %s", i+1, rule))
			lines = append(lines, fmt.Sprintf("- [ ] `%s:%d`", file, startLine))
			if desc != "" {
				lines = append(lines, "- Context: "+desc)
			}
			if secret != "" {
				lines = append(lines, "- Match: `"+secret+"`")
			}
			lines = append(lines, "")
		}
	}

	todoPath := filepath.Join(repoPath, "LEAKS_TODO.md")
	if err := os.WriteFile(todoPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	return todoPath, nil
}

func anyToString(v any) string {
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}

func anyToInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func countGitleaksFindings(reportPath string) int {
	b, err := os.ReadFile(reportPath)
	if err != nil {
		return 0
	}
	var arr []map[string]any
	if err := json.Unmarshal(b, &arr); err == nil {
		return len(arr)
	}
	var obj struct {
		Findings []any `json:"findings"`
	}
	if err := json.Unmarshal(b, &obj); err == nil {
		return len(obj.Findings)
	}
	return 0
}

func formatGitleaksReport(msg gitleaksDoneMsg) string {
	status := "FAILED"
	if msg.err == nil && msg.ok {
		status = "CLEAN"
	} else if msg.err == nil {
		status = "LEAKS FOUND"
	}
	body := msg.output
	if strings.TrimSpace(body) == "" {
		body = "(no output)"
	}
	lines := strings.Split(body, "\n")
	if len(lines) > 40 {
		lines = append(lines[:40], "... (truncated)")
	}
	return titleStyle.Render("Gitleaks Report") + "\n" +
		fmt.Sprintf("Mode: %s\nStatus: %s\nLeaks: %d\nReport: %s\n\n%s", msg.mode, status, msg.leakCount, msg.reportPath, strings.Join(lines, "\n"))
}
