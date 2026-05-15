package main

import (
	"bufio"
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
	"text/tabwriter"
)

type RepoStatus struct {
	Path      string
	Branch    string
	Clean     bool
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

func main() {
	configPath := flag.String("config", "repos.config.json", "Path to config file")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-config repos.config.json]\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Scans folders from config for Git repositories and prints a status table.")
	}
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	roots := cfg.Folders
	if len(roots) == 0 {
		fmt.Fprintf(os.Stderr, "config error: no folders configured in %s\n", *configPath)
		os.Exit(2)
	}

	repos, err := findGitRepos(roots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(1)
	}

	if len(repos) == 0 {
		fmt.Println("No Git repositories found.")
		return
	}

	statuses := make([]RepoStatus, 0, len(repos))
	for _, repo := range repos {
		statuses = append(statuses, getRepoStatus(repo))
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Path < statuses[j].Path
	})

	printTable(statuses)
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
				// Skip unreadable directories/files and continue.
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			if d.Name() == ".git" {
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
	status := RepoStatus{Path: repoPath, Clean: true}

	branch, err := gitOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Branch = strings.TrimSpace(branch)

	aheadBehind, err := gitOutput(repoPath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err == nil {
		fields := strings.Fields(aheadBehind)
		if len(fields) == 2 {
			// left=ahead, right=behind for HEAD...upstream
			fmt.Sscanf(fields[0], "%d", &status.Ahead)
			fmt.Sscanf(fields[1], "%d", &status.Behind)
		}
	}

	porcelain, err := gitOutput(repoPath, "status", "--porcelain")
	if err != nil {
		status.Error = err.Error()
		return status
	}

	scanner := bufio.NewScanner(strings.NewReader(porcelain))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		x := line[0]
		y := line[1]

		if x != ' ' && x != '?' {
			status.Staged = true
			status.Clean = false
		}
		if y != ' ' {
			status.Modified = true
			status.Clean = false
		}
		if x == '?' && y == '?' {
			status.Untracked = true
			status.Clean = false
		}
	}

	if err := scanner.Err(); err != nil {
		status.Error = err.Error()
	}

	return status
}

func gitOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			return "", err
		}
		return "", errors.New(trimmed)
	}
	return string(out), nil
}

func printTable(statuses []RepoStatus) {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tBRANCH\tSTATE\tAHEAD\tBEHIND\tERROR")

	for _, s := range statuses {
		state := "clean"
		if s.Staged || s.Modified || s.Untracked {
			parts := make([]string, 0, 3)
			if s.Staged {
				parts = append(parts, "staged")
			}
			if s.Modified {
				parts = append(parts, "modified")
			}
			if s.Untracked {
				parts = append(parts, "untracked")
			}
			state = strings.Join(parts, ",")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n", s.Path, s.Branch, state, s.Ahead, s.Behind, s.Error)
	}

	_ = w.Flush()
}
