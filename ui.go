package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeList mode = iota
	modeAddChoice
	modeAddSecret // paste URI or base32
	modeAddQRFile // path to screenshot
	modeAddMeta   // issuer/account for raw secrets
	modeEdit
	modeConfirmDel
)

type tickMsg time.Time

type model struct {
	vault    *Vault
	path     string
	password string
	mode     mode
	cursor   int
	input    string
	pending  Entry
	editField int // 0 = issuer, 1 = account
	status   string
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	codeStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func newModel(v *Vault, path, password string) model {
	return model{vault: v, path: path, password: password}
}

func (m model) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) save() {
	if err := SaveVault(m.path, m.password, m.vault); err != nil {
		m.status = errStyle.Render("save failed: " + err.Error())
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, tick()

	case tea.KeyMsg:
		switch m.mode {

		case modeList:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.vault.Entries)-1 {
					m.cursor++
				}
			case "a":
				m.mode, m.input, m.status = modeAddChoice, "", ""
			case "d":
				if len(m.vault.Entries) > 0 {
					m.mode = modeConfirmDel
				}
			case "s": // scan screen for QR
				text, err := QRFromScreen()
				if err != nil {
					m.status = errStyle.Render(err.Error())
					break
				}
				e, err := ParseInput(text)
				if err != nil {
					m.status = errStyle.Render(err.Error())
					break
				}
				// don't save yet — let the user confirm/edit the name first
				m.pending = e
				m.input = e.Issuer // prefill with whatever the QR provided
				m.mode = modeAddMeta
			case "e": // edit selected entry
				if len(m.vault.Entries) > 0 {
					m.editField = 0
					m.input = m.vault.Entries[m.cursor].Issuer
					m.mode = modeEdit
				}
			}
		case modeAddChoice:
			switch msg.String() {
			case "1":
				m.mode, m.input = modeAddSecret, ""
			case "2":
				m.mode, m.input = modeAddQRFile, ""
			case "esc":
				m.mode = modeList
			}

		case modeAddSecret, modeAddQRFile, modeAddMeta, modeEdit:
			switch msg.String() {
			case "esc":
				m.mode = modeList
			case "enter":
				m = m.submitInput()
			case "backspace":
				if r := []rune(m.input); len(r) > 0 {
					m.input = string(r[:len(r)-1])
				}
			default:
				if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
					m.input += msg.String()
				}
			}

		case modeConfirmDel:
			switch msg.String() {
			case "y":
				m.vault.Entries = append(
					m.vault.Entries[:m.cursor],
					m.vault.Entries[m.cursor+1:]...)
				if m.cursor >= len(m.vault.Entries) && m.cursor > 0 {
					m.cursor--
				}
				m.save()
				m.mode = modeList
			case "n", "esc":
				m.mode = modeList
			}
		}
	}
	return m, nil	
}

func (m model) submitInput() model {
	switch m.mode {
	case modeAddSecret:
		e, err := ParseInput(m.input)
		if err != nil {
			m.status = errStyle.Render(err.Error())
			m.mode = modeList
			return m
		}
		if e.Issuer == "" { // raw secret needs a label
			m.pending, m.mode, m.input = e, modeAddMeta, ""
			return m
		}
		m.vault.Entries = append(m.vault.Entries, e)
	case modeAddQRFile:
		text, err := QRFromFile(strings.TrimSpace(m.input))
		if err != nil {
			m.status = errStyle.Render(err.Error())
			m.mode = modeList
			return m
		}
		e, err := ParseInput(text)
		if err != nil {
			m.status = errStyle.Render(err.Error())
			m.mode = modeList
			return m
		}
		m.vault.Entries = append(m.vault.Entries, e)
	case modeAddMeta:
		m.pending.Issuer = strings.TrimSpace(m.input)
		m.vault.Entries = append(m.vault.Entries, m.pending)
	case modeEdit:
		e := &m.vault.Entries[m.cursor]
		switch m.editField {
		case 0: // issuer
			if name := strings.TrimSpace(m.input); name != "" {
				e.Issuer = name
			}
			m.editField = 1
			m.input = e.Account // prefill next field
			return m           // stay in edit mode, skip save-and-exit below
		case 1: // account
			e.Account = strings.TrimSpace(m.input) // blank is allowed here
		}
	}
	
	m.save()
	m.mode = modeList
	
	return m
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("  goauth — TOTP vault") + "\n\n")

	switch m.mode {
	case modeAddChoice:
		b.WriteString("  Add entry:\n\n  [1] paste otpauth:// URI or secret key\n")
		b.WriteString("  [2] QR code from image file\n\n")
		b.WriteString(dimStyle.Render("  (or press 's' from the list to scan your screen)  esc: cancel"))
	case modeAddSecret:
		b.WriteString("  Paste URI or Base32 secret:\n\n  > " + m.input + "█\n")
	case modeAddQRFile:
		b.WriteString("  Path to screenshot image:\n\n  > " + m.input + "█\n")
	case modeAddMeta:
		b.WriteString("  Name for this entry (e.g. GitHub):\n\n  > " + m.input + "█\n")
	case modeEdit:
		label := [...]string{"Name", "Account (optional)"}[m.editField]
		b.WriteString(fmt.Sprintf("  Edit %s:\n\n  > %s█\n", label, m.input))
		b.WriteString(dimStyle.Render("\n  enter: next/save  esc: cancel"))
	case modeConfirmDel:
		e := m.vault.Entries[m.cursor]
		b.WriteString(fmt.Sprintf("  Delete %s (%s)? [y/n]\n", e.Issuer, e.Account))
	default:
		now := time.Now()
		if len(m.vault.Entries) == 0 {
			b.WriteString(dimStyle.Render("  no entries yet — press 'a' to add one\n"))
		}
		for i, e := range m.vault.Entries {
			prefix := "  "
			name := fmt.Sprintf("%-20s", e.Issuer)
			if e.Account != "" {
				name = fmt.Sprintf("%-20s %s", e.Issuer, dimStyle.Render(e.Account))
			}
			line := fmt.Sprintf("%s%s  %s  %s",
				prefix, codeStyle.Render(e.Code(now)), name,
				progressBar(e.SecondsLeft(now), e.Period))
			if i == m.cursor {
				line = selStyle.Render("▸ ") + line[2:]
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n" + dimStyle.Render("  a: add  s: scan screen  e: rename  d: delete  ↑/↓: move  q: quit"))
	}
	if m.status != "" {
		b.WriteString("\n\n  " + m.status)
	}
	return b.String()
}

func progressBar(left, period int) string {
	width := 10
	filled := left * width / period
	return dimStyle.Render("[" + strings.Repeat("█", filled) +
		strings.Repeat("░", width-filled) + fmt.Sprintf("] %2ds", left))
}
