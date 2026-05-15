# GopherHole

A terminal UI for scanning folders, discovering Git repositories, and viewing their status in one place.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## Features

- Recursively scans configured folders for Git repos
- Skips heavy dirs like `node_modules`
- Groups repos by status (changes, untracked, clean, errors)
- Shows branch overview (ahead/behind + current branch working-tree status)
- Fast keyboard navigation
- Optional integrations:
  - `lazygit`
  - `nvim`
  - `yazi`
  - shell in repo dir

---

## Requirements

- Go 1.22+
- Git
- Optional: `lazygit`, `nvim`, `yazi`

---

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

Then place it somewhere in your `PATH`, e.g.:

```bash
mkdir -p "$HOME/.local/bin"
cp gopherhole "$HOME/.local/bin/gopherhole"
chmod +x "$HOME/.local/bin/gopherhole"
```

---

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
  "folders": [
    "/path/to/work",
    "/path/to/personal"
  ]
}
```

You can override config location:

```bash
gopherhole -config /path/to/repos.config.json
```

---

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

---

## Cross-compile

```bash
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o gopherhole-linux-amd64 .

# Linux arm64
GOOS=linux GOARCH=arm64 go build -o gopherhole-linux-arm64 .

# macOS arm64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o gopherhole-darwin-arm64 .

# macOS amd64 (Intel)
GOOS=darwin GOARCH=amd64 go build -o gopherhole-darwin-amd64 .
```

---

## Notes

Branch file-status accuracy is complete for the currently checked-out branch.
For non-checked-out branches, commit divergence (ahead/behind) is shown.