# GopherHole

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![TUI](https://img.shields.io/badge/UI-Bubble%20Tea%20%2B%20Lip%20Gloss-ff69b4)](https://github.com/charmbracelet/bubbletea)

A terminal UI for scanning folders, discovering Git repositories, and viewing their status in one place.

## Why GopherHole?

If you juggle many repos, checking each one manually is slow and noisy. GopherHole gives you a single dashboard to:

- find repos across multiple root folders
- instantly spot dirty/untracked/clean states
- jump straight into tools (`lazygit`, `nvim`, `yazi`, shell)

## Demo

> Add your screenshots/GIFs to `docs/assets/` and update these links.

![GopherHole Screenshot](docs/assets/screenshot.png)

<!-- ![GopherHole Demo GIF](docs/assets/demo.gif) -->

## Features

- Recursively scans configured folders for Git repos
- Skips heavy dirs like `node_modules`
- Groups repos by status (changes, untracked, clean, errors)
- Branch overview (ahead/behind + current branch working-tree status)
- Fast keyboard navigation
- Tool integrations:
  - `g` → `lazygit`
  - `v` → `nvim`
  - `y` → `yazi`
  - `o` → shell in repo dir

## Requirements

- Go 1.22+
- Git
- Optional: `lazygit`, `nvim`, `yazi`

## Install

### Recommended

```bash
./install.sh
```

Optional custom install dir:

```bash
INSTALL_DIR=/usr/local/bin ./install.sh
```

### Manual build

```bash
go build -o gopherhole .
```

Then place it somewhere in your `PATH`:

```bash
mkdir -p "$HOME/.local/bin"
cp gopherhole "$HOME/.local/bin/gopherhole"
chmod +x "$HOME/.local/bin/gopherhole"
```

## Configuration

Default config path (macOS + Linux):

```text
~/.config/gopherhole/repos.config.json
```

Initialize it automatically:

```bash
gopherhole init
```

Or create manually:

```bash
mkdir -p "$HOME/.config/gopherhole"
cp repos.config.example.json "$HOME/.config/gopherhole/repos.config.json"
```

Config format:

```json
{
  "folders": ["/path/to/work", "/path/to/personal"]
}
```

Override config location:

```bash
gopherhole -config /path/to/repos.config.json
```

## Usage

```bash
gopherhole
```

### Keybindings

- `↑/↓` or `j/k` — move selection
- `r` — rescan
- `g` — open selected repo in `lazygit`
- `v` — open selected repo in `nvim`
- `y` — open selected repo in `yazi`
- `o` — open shell in selected repo
- `q` — quit

## Cross-compile

```bash
GOOS=linux GOARCH=amd64 go build -o gopherhole-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o gopherhole-linux-arm64 .
GOOS=darwin GOARCH=arm64 go build -o gopherhole-darwin-arm64 .
GOOS=darwin GOARCH=amd64 go build -o gopherhole-darwin-amd64 .
```

## Notes

Branch file-status accuracy is complete for the currently checked-out branch.
For non-checked-out branches, commit divergence (ahead/behind) is shown.
