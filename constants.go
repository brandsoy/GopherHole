package main

const (
	keyQuit      = "q"
	keyQuitAlt   = "ctrl+c"
	keyUp        = "up"
	keyUpAlt     = "k"
	keyDown      = "down"
	keyDownAlt   = "j"
	keyRefresh   = "r"
	keyClear     = "c"
	keyPopupOpen = "p"
	keyTodoFile  = "t"
	keyEnter     = "enter"
	keyEsc       = "esc"

	keyGitMenu    = "g"
	keyNvim       = "v"
	keyConfigNvim = "e"
	keyYazi       = "y"
	keyShell      = "o"

	keyGitMenuLazygit = "g"
	keyGitMenuLeakL   = "l"
	keyGitMenuLeakR   = "L"
	keyGitMenuReport  = "r"

	keyBulkLeakAll = "a"

	keyConfirmYes = "y"
	keyConfirmNo  = "n"
)

const (
	detailPanelWidth = 70
	popupPanelWidth  = 100
)

const helpText = "↑/↓ move • g git menu • p open report • a gitleaks-all-local • v vim repo • e open config • y yazi • o shell • r rescan repos • q quit"
