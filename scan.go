package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

		latestLeaks := loadLatestLeakCounts()
		for i := range statuses {
			if leak, ok := latestLeaks[statuses[i].Path]; ok {
				statuses[i].LocalLeakScanned = leak.LocalScanned
				statuses[i].LocalLeakCount = leak.LocalCount
				statuses[i].RemoteLeakScanned = leak.RemoteScanned
				statuses[i].RemoteLeakCount = leak.RemoteCount
				statuses[i].LatestLocalReport = leak.LocalReport
				statuses[i].LatestRemoteReport = leak.RemoteReport
				statuses[i].LatestAnyReport = leak.AnyReport
			}
		}
		sort.Slice(statuses, func(i, j int) bool {
			gi, gj := statusGroupRank(statuses[i]), statusGroupRank(statuses[j])
			if gi != gj {
				return gi < gj
			}
			return strings.ToLower(statuses[i].Path) < strings.ToLower(statuses[j].Path)
		})
		return scanDoneMsg{statuses: statuses}
	}
}

func findGitRepos(roots []string) ([]string, error) {
	seen := map[string]struct{}{}
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
			switch d.Name() {
			case "node_modules":
				return filepath.SkipDir
			case ".git":
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

	if ab, err := gitOutput(repoPath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}"); err == nil {
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

	if branches, berr := listBranches(repoPath, s.Branch, s.Staged, s.Modified, s.Untracked); berr == nil {
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
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		b := BranchStatus{Name: name, IsCurrent: name == current}
		if ab, err := gitOutput(repoPath, "rev-list", "--left-right", "--count", b.Name+"..."+b.Name+"@{upstream}"); err == nil {
			fields := strings.Fields(ab)
			if len(fields) == 2 {
				fmt.Sscanf(fields[0], "%d", &b.Ahead)
				fmt.Sscanf(fields[1], "%d", &b.Behind)
			}
		}
		if b.IsCurrent {
			b.Staged, b.Modified, b.Untracked = currentStaged, currentModified, currentUntracked
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
