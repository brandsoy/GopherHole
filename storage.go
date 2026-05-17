package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type reportIndexEntry struct {
	CreatedAt string `json:"created_at"`
	RepoPath  string `json:"repo_path"`
	Mode      string `json:"mode"`
	Leaks     int    `json:"leaks"`
	Path      string `json:"path"`
}

func reportsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "reports"
	}
	return filepath.Join(home, ".local", "share", "gopherhole", "reports")
}

func ensureReportsDir() error {
	return os.MkdirAll(reportsDir(), 0o755)
}

func reportFilePath(repoPath, mode string) string {
	ts := time.Now().Format("20060102-150405")
	slug := strings.ToLower(filepath.Base(repoPath))
	slug = strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-").Replace(slug)
	if slug == "" {
		slug = "repo"
	}
	return filepath.Join(reportsDir(), ts+"_"+slug+"_"+mode+".json")
}

type latestLeakData struct {
	LocalScanned  bool
	LocalCount    int
	RemoteScanned bool
	RemoteCount   int
	LocalReport   string
	RemoteReport  string
	AnyReport     string
}

func loadLatestLeakCounts() map[string]latestLeakData {
	result := map[string]latestLeakData{}

	f, err := os.Open(filepath.Join(reportsDir(), "index.ndjson"))
	if err != nil {
		return result
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var e reportIndexEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		v := result[e.RepoPath]
		if e.Mode == "local" {
			v.LocalScanned = true
			v.LocalCount = e.Leaks
			v.LocalReport = e.Path
		} else if e.Mode == "remote" {
			v.RemoteScanned = true
			v.RemoteCount = e.Leaks
			v.RemoteReport = e.Path
		}
		v.AnyReport = e.Path
		result[e.RepoPath] = v
	}
	return result
}

func appendReportIndex(entry reportIndexEntry) {
	_ = os.MkdirAll(reportsDir(), 0o755)
	f, err := os.OpenFile(filepath.Join(reportsDir(), "index.ndjson"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
}
