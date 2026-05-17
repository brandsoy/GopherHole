package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	m := model{
		configPath:    *configPath,
		loading:       true,
		busy:          true,
		busyLabel:     "Scanning repositories",
		busyStartedAt: time.Now(),
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
