# GopherHole

Scan one or more root folders for Git repositories and view repo status in a Bubble Tea TUI.

## Config file location

By default, GopherHole reads config from:

- **macOS:** `~/.config/gopherhole/repos.config.json`
- **Linux:** `~/.config/gopherhole/repos.config.json`

You can override with `-config /path/to/repos.config.json`.

## Initialize config

Create a starter config at the default location:

```bash
gopherhole init
```

## Config format

```json
{
  "folders": [
    "/path/to/work",
    "/path/to/personal"
  ]
}
```

Create it from the example:

### macOS / Linux

```bash
mkdir -p "$HOME/.config/gopherhole"
cp repos.config.example.json "$HOME/.config/gopherhole/repos.config.json"
```

## Build

### Build for current OS

```bash
go build -o gopherhole .
```

### Cross-compile

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

## Install script (recommended)

```bash
./install.sh
```

Optional custom install dir:

```bash
INSTALL_DIR=/usr/local/bin ./install.sh
```

## Install as terminal command

Put the binary somewhere in your `PATH`.

### User-local install

```bash
mkdir -p "$HOME/.local/bin"
cp gopherhole "$HOME/.local/bin/gopherhole"
chmod +x "$HOME/.local/bin/gopherhole"
```

Make sure `~/.local/bin` is in `PATH`.

### System-wide install

```bash
sudo cp gopherhole /usr/local/bin/gopherhole
sudo chmod +x /usr/local/bin/gopherhole
```

## Run

```bash
gopherhole
```

Or with custom config:

```bash
gopherhole -config /path/to/repos.config.json
```

## TUI keys

- `↑/↓` or `j/k`: move
- `g`: open selected repo in `lazygit` (if installed)
- `r`: rescan
- `q`: quit
