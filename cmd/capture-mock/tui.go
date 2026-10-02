package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/automuteus/automuteus/v8/pkg/game"
)

type screen int

const (
	screenConnect screen = iota
	screenConnecting
	screenName
	screenMenu
	screenCheckpoint
	screenReport
	screenManual
	screenForm
)

const defaultPlayerName = "Player One"

var manualItems = []string{"Lobby details", "Game phase", "Player event", "Gameover", "Back"}

// model is the whole UI. Events are sent synchronously from Update: Galactus
// is local, and it keeps the session free of concurrent access.
type model struct {
	send          func(tea.Msg)
	screen        screen
	width, height int

	input    textinput.Model
	inputErr string

	code string
	s    *session
	you  string

	cursor     int
	run        *scenarioRun
	checkpoint string
	notes      []string // operator instructions before the checkpoint
	runErr     error

	form   *huh.Form
	submit func() error

	entries []logEntry
	logView viewport.Model

	// err is reported when the program exits.
	err error
}

func newModel(send func(tea.Msg)) *model {
	m := &model{send: send, logView: viewport.New()}
	m.input = newInput("ABCDEFGH or aucapture://localhost:8123/ABCDEFGH?insecure")
	return m
}

func newInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = placeholder
	ti.Focus()
	return ti
}

func (m *model) Init() tea.Cmd { return textinput.Blink }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil
	case logEntry:
		m.appendLog(msg)
		return m, nil
	case fatalMsg:
		m.err = msg.err
		return m, tea.Quit
	case connectedMsg:
		m.s = newSession(msg.client)
		m.appendLog(logEntry{entryInfo, "Connect code sent"})
		m.screen = screenName
		m.input = newInput(defaultPlayerName)
		m.layout()
		return m, textinput.Blink
	case connectFailedMsg:
		m.appendLog(logEntry{entryError, msg.err.Error()})
		m.inputErr, m.screen = "Could not reach Galactus; is it running? Details are in the event log.", screenConnect
		return m, textinput.Blink
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "pgup":
			m.logView.PageUp()
			return m, nil
		case "pgdown":
			m.logView.PageDown()
			return m, nil
		}
	}
	switch m.screen {
	case screenConnect, screenName:
		return m, m.updateInput(msg)
	case screenForm:
		return m, m.updateForm(msg)
	}
	if press, ok := msg.(tea.KeyPressMsg); ok {
		switch m.screen {
		case screenMenu:
			return m, m.updateMenu(press.String())
		case screenCheckpoint:
			return m, m.updateCheckpoint(press.String())
		case screenReport:
			if press.String() == "enter" || press.String() == "esc" {
				m.screen, m.cursor = screenMenu, 0
			}
		case screenManual:
			return m, m.updateManual(press.String())
		}
	}
	return m, nil
}

func (m *model) updateInput(msg tea.Msg) tea.Cmd {
	if press, ok := msg.(tea.KeyPressMsg); ok && press.String() == "enter" {
		if m.screen == screenName {
			m.you = strings.TrimSpace(m.input.Value())
			if m.you == "" {
				m.you = defaultPlayerName
			}
			m.screen, m.cursor = screenMenu, 0
			return nil
		}
		host, code, err := parseConnection(m.input.Value())
		if err != nil {
			m.inputErr = err.Error()
			return nil
		}
		m.inputErr, m.code, m.screen = "", code, screenConnecting
		m.appendLog(logEntry{entryInfo, "Connecting to " + host})
		return connect(host, code, m.send)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *model) updateMenu(k string) tea.Cmd {
	items := len(scenarios) + 2
	switch k {
	case "up", "k":
		m.cursor = (m.cursor + items - 1) % items
	case "down", "j":
		m.cursor = (m.cursor + 1) % items
	case "m":
		m.cursor = len(scenarios)
		return m.updateMenu("enter")
	case "q", "esc":
		return tea.Quit
	case "enter":
		switch {
		case m.cursor < len(scenarios):
			sc := scenarios[m.cursor]
			m.run = newRun(m.s, sc, m.you)
			m.appendLog(logEntry{entryInfo, "== " + sc.name + " =="})
			return m.advance()
		case m.cursor == len(scenarios):
			m.screen, m.cursor = screenManual, 0
		default:
			return tea.Quit
		}
	default:
		if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(scenarios) {
			m.cursor = n - 1
			return m.updateMenu("enter")
		}
	}
	return nil
}

// advance plays the scenario up to its next checkpoint, or to the report.
func (m *model) advance() tea.Cmd {
	entries, checkpoint, err := m.run.advance()
	m.notes = nil
	for _, e := range entries {
		m.appendLog(e)
		if e.kind == entryNote {
			m.notes = append(m.notes, e.text)
		}
	}
	if err != nil {
		m.err = err
		return tea.Quit
	}
	if checkpoint == "" {
		m.finish(nil)
		return nil
	}
	m.checkpoint, m.screen = checkpoint, screenCheckpoint
	return nil
}

func (m *model) updateCheckpoint(k string) tea.Cmd {
	switch k {
	case "y", "n":
		observed := k == "y"
		m.run.answer(observed)
		if observed {
			m.appendLog(logEntry{entryPass, m.checkpoint})
		} else {
			m.appendLog(logEntry{entryFail, m.checkpoint})
		}
		return m.advance()
	case "q", "esc":
		if err := m.run.abort(); !errors.Is(err, errAborted) {
			m.err = err
			return tea.Quit
		}
		m.appendLog(logEntry{entrySent, "state MENU"})
		m.finish(errAborted)
	}
	return nil
}

func (m *model) finish(err error) {
	m.runErr, m.screen = err, screenReport
	_, total := m.run.progress()
	summary := fmt.Sprintf("%s: %d/%d checks observed", m.run.sc.name, tally(m.run.results), total)
	if err != nil {
		summary += fmt.Sprintf(" (%v)", err)
	}
	m.appendLog(logEntry{entryInfo, summary})
}

func (m *model) updateManual(k string) tea.Cmd {
	switch k {
	case "up", "k":
		m.cursor = (m.cursor + len(manualItems) - 1) % len(manualItems)
	case "down", "j":
		m.cursor = (m.cursor + 1) % len(manualItems)
	case "q", "esc":
		m.screen, m.cursor = screenMenu, len(scenarios)
	case "enter":
		switch m.cursor {
		case 0:
			return m.lobbyForm()
		case 1:
			return m.phaseForm()
		case 2:
			return m.playerForm()
		case 3:
			return m.gameoverForm()
		default:
			return m.updateManual("esc")
		}
	}
	return nil
}

func (m *model) updateForm(msg tea.Msg) tea.Cmd {
	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)
	switch m.form.State {
	case huh.StateAborted:
		m.screen = screenManual
	case huh.StateCompleted:
		done := m.form
		// Stop on a failed send: continuing would hide a partially sent sequence.
		if err := m.submit(); err != nil {
			m.err = err
			return tea.Quit
		}
		// submit may have opened a follow-up form instead of sending.
		if m.form == done {
			m.screen = screenManual
		} else {
			return m.form.Init()
		}
	}
	return cmd
}

func (m *model) openForm(submit func() error, groups ...*huh.Group) tea.Cmd {
	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
	m.form = huh.NewForm(groups...).WithKeyMap(keys).WithTheme(huh.ThemeFunc(huh.ThemeCharm)).WithWidth(m.mainWidth() - 4)
	m.submit, m.screen = submit, screenForm
	return m.form.Init()
}

// sendStep sends a manual event built with the scenario helpers, so it is
// encoded and logged exactly as a scenario would.
func (m *model) sendStep(st step) error {
	if err := st.send(m.s, m.you); err != nil {
		return err
	}
	m.appendLog(logEntry{entrySent, st.label})
	return nil
}

func selectOf[T comparable](title string, opts []option[T], value *T) *huh.Select[T] {
	choices := make([]huh.Option[T], len(opts))
	for i, o := range opts {
		choices[i] = huh.NewOption(o.label, o.value)
	}
	return huh.NewSelect[T]().Title(title).Options(choices...).Value(value)
}

func (m *model) lobbyForm() tea.Cmd {
	code, region, playMap := "TESTCODE", game.NA, game.SKELD
	return m.openForm(func() error {
		return m.sendStep(lobby(code, region, playMap))
	}, huh.NewGroup(
		huh.NewInput().Title("Lobby code").Value(&code).Validate(huh.ValidateNotEmpty()),
		selectOf("Region", regionOptions, &region),
		selectOf("Map", mapOptions, &playMap),
	))
}

func (m *model) phaseForm() tea.Cmd {
	p := game.TASKS
	return m.openForm(func() error {
		return m.sendStep(phase(p))
	}, huh.NewGroup(selectOf("Phase", phaseOptions, &p)))
}

func (m *model) gameoverForm() tea.Cmd {
	result := game.HumansByVote
	return m.openForm(func() error {
		return m.sendStep(gameover(result))
	}, huh.NewGroup(selectOf("Game result", resultOptions, &result)))
}

// playerForm asks for the action and name first, so that the details form can
// default to that player's current color and role and the action's flags.
func (m *model) playerForm() tea.Cmd {
	p := game.Player{Action: game.JOINED, Name: m.you}
	return m.openForm(func() error {
		p.Color = game.Red
		for _, seen := range m.s.view.players {
			if seen.Name == p.Name {
				p.Color = seen.Color
			}
		}
		p.IsDead = p.Action == game.DIED || p.Action == game.EXILED
		p.Disconnected = p.Action == game.DISCONNECTED
		impostor := m.s.isImpostor(p.Name)
		m.openForm(func() error {
			if err := m.s.player(p, impostor); err != nil {
				return err
			}
			m.appendLog(logEntry{entrySent, playerLabel(p.Name, p.Action, p.Color)})
			return nil
		}, huh.NewGroup(
			selectOf("Color", colorOptions, &p.Color).Height(8),
			huh.NewConfirm().Title("Is dead?").Value(&p.IsDead),
			huh.NewConfirm().Title("Disconnected?").Value(&p.Disconnected),
			huh.NewConfirm().Title("Is impostor?").Description("Sent at gameover").Value(&impostor),
		))
		return nil
	}, huh.NewGroup(
		selectOf("Player action", actionOptions, &p.Action),
		huh.NewInput().Title("Player name").Value(&p.Name).Validate(huh.ValidateNotEmpty()),
	))
}

func (m *model) appendLog(e logEntry) {
	m.entries = append(m.entries, e)
	follow := m.logView.AtBottom()
	m.logView.SetContent(m.renderLog())
	if follow {
		m.logView.GotoBottom()
	}
}

// Layout

const panelWidth = 36

var (
	accent   = lipgloss.Color("#7D56F4")
	subtle   = lipgloss.Color("#6C7086")
	good     = lipgloss.Color("#40A02B")
	bad      = lipgloss.Color("#E64553")
	warn     = lipgloss.Color("#DF8E1D")
	galactus = lipgloss.Color("#1E88E5")

	paneStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(subtle).Padding(0, 1)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	dimStyle   = lipgloss.NewStyle().Foreground(subtle)
	errStyle   = lipgloss.NewStyle().Foreground(bad)
	badge      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(accent).Padding(0, 1)
	checkStyle = lipgloss.NewStyle().Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(warn).PaddingLeft(1)

	entryStyles = map[entryKind]struct {
		prefix string
		style  lipgloss.Style
	}{
		entrySent:     {"→ ", dimStyle},
		entryNote:     {"» ", lipgloss.NewStyle().Foreground(warn)},
		entryGalactus: {"◆ ", lipgloss.NewStyle().Foreground(galactus)},
		entryInfo:     {"", lipgloss.NewStyle().Bold(true)},
		entryPass:     {"✓ ", lipgloss.NewStyle().Foreground(good)},
		entryFail:     {"✗ ", lipgloss.NewStyle().Foreground(bad)},
		entryError:    {"! ", errStyle},
	}

	// Among Us player colors, for the roster swatches.
	playerColors = map[int]string{
		game.Red: "#C51111", game.Blue: "#132ED1", game.Green: "#117F2D", game.Pink: "#ED54BA",
		game.Orange: "#EF7D0D", game.Yellow: "#F5F557", game.Black: "#3F474E", game.White: "#D6E0F0",
		game.Purple: "#6B2FBB", game.Brown: "#71491E", game.Cyan: "#38FEDC", game.Lime: "#50EF39",
		game.Maroon: "#6B2B3C", game.Rose: "#ECC0D3", game.Banana: "#FFFEBE", game.Gray: "#708496",
		game.Tan: "#928776", game.Coral: "#EC7578",
	}
)

func (m *model) showPanel() bool { return m.width >= 80 }

func (m *model) mainWidth() int {
	if m.showPanel() {
		return m.width - panelWidth
	}
	return m.width
}

func (m *model) logHeight() int { return max(6, min(14, m.height/3)) }

func (m *model) bodyHeight() int { return max(3, m.height-m.logHeight()-2) }

func (m *model) layout() {
	m.logView.SetWidth(max(1, m.width-4))
	m.logView.SetHeight(max(1, m.logHeight()-2))
	m.input.SetWidth(max(10, m.mainWidth()-8))
	if m.form != nil {
		m.form = m.form.WithWidth(m.mainWidth() - 4)
	}
	m.logView.SetContent(m.renderLog())
	m.logView.GotoBottom()
}

func (m *model) renderLog() string {
	lines := make([]string, len(m.entries))
	for i, e := range m.entries {
		s := entryStyles[e.kind]
		lines[i] = s.style.Width(m.logView.Width()).Render(s.prefix + e.text)
	}
	return strings.Join(lines, "\n")
}

func (m *model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.WindowTitle = "capture-mock"
	if m.width == 0 {
		return v
	}
	body := m.pane(m.mainView(m.mainWidth()-4), m.mainWidth())
	if m.showPanel() {
		panel := m.pane(m.panelView(panelWidth-4), panelWidth)
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, panel)
	}
	events := paneStyle.Width(m.width).Height(m.logHeight()).Render(m.logView.View())
	v.SetContent(lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body, events, m.helpView()))
	return v
}

// pane boxes content to the body height, cutting overflow inside the border.
func (m *model) pane(content string, width int) string {
	lines := strings.Split(content, "\n")
	lines = lines[:min(len(lines), m.bodyHeight()-2)]
	return paneStyle.Width(width).Height(m.bodyHeight()).Render(strings.Join(lines, "\n"))
}

func (m *model) headerView() string {
	status := dimStyle.Render("not connected")
	if m.s != nil {
		status = fmt.Sprintf("%s  code %s", galactusHost, m.code)
		if m.you != "" {
			status += "  you: " + m.you
		}
	}
	return badge.Render("capture-mock") + " " + status
}

func (m *model) helpView() string {
	var keys string
	switch m.screen {
	case screenConnect, screenName:
		keys = "enter confirm"
	case screenMenu:
		keys = "↑/↓ choose  enter start  1-9 scenario  m manual  q quit"
	case screenCheckpoint:
		keys = "y observed  n not observed  q stop scenario"
	case screenReport:
		keys = "enter back to scenarios"
	case screenManual:
		keys = "↑/↓ choose  enter open  esc back"
	case screenForm:
		keys = "enter next  shift+tab back  esc cancel"
	}
	return dimStyle.Render(" " + keys + "  pgup/pgdn scroll log  ctrl+c quit")
}

func (m *model) mainView(width int) string {
	wrap := lipgloss.NewStyle().Width(width)
	var b strings.Builder
	switch m.screen {
	case screenConnect:
		b.WriteString(titleStyle.Render("Connect") + "\n\n")
		b.WriteString(wrap.Render("Create a game with /new in your development bot, then paste its connect code or aucapture:// link. The connect code is not the Among Us lobby code.") + "\n\n")
		b.WriteString(m.input.View())
		if m.inputErr != "" {
			b.WriteString("\n\n" + errStyle.Width(width).Render(m.inputErr))
		}
	case screenConnecting:
		b.WriteString(titleStyle.Render("Connecting") + "\n\n" + dimStyle.Render("Dialing "+galactusHost+"…"))
	case screenName:
		b.WriteString(titleStyle.Render("Your player") + "\n\n")
		b.WriteString(wrap.Render("Name of the player you will link to. Your Discord username or nickname links automatically; any other name can be linked from the status message's color dropdown.") + "\n\n")
		b.WriteString(m.input.View())
	case screenMenu:
		b.WriteString(titleStyle.Render("Scenarios") + "\n\n")
		for i, sc := range scenarios {
			b.WriteString(m.menuItem(i, fmt.Sprintf("%d  %s", i+1, sc.name)) + "\n")
		}
		b.WriteString(m.menuItem(len(scenarios), "m  Send events manually") + "\n")
		b.WriteString(m.menuItem(len(scenarios)+1, "q  Quit") + "\n")
		if m.cursor < len(scenarios) {
			b.WriteString("\n" + dimStyle.Width(width).Render(scenarios[m.cursor].description))
		}
	case screenCheckpoint:
		answered, total := m.run.progress()
		b.WriteString(titleStyle.Render(m.run.sc.name) + dimStyle.Render(fmt.Sprintf("  check %d of %d", answered+1, total)) + "\n\n")
		for _, note := range m.notes {
			s := entryStyles[entryNote]
			b.WriteString(s.style.Width(width).Render(s.prefix+note) + "\n\n")
		}
		b.WriteString(checkStyle.Width(width).Render(m.checkpoint) + "\n\n")
		b.WriteString("Did the bot do this in Discord?\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(good).Render("y") + " observed   " +
			lipgloss.NewStyle().Foreground(bad).Render("n") + " not observed   " +
			dimStyle.Render("q stop (sends the menu phase so nobody stays muted)"))
	case screenReport:
		_, total := m.run.progress()
		observed := tally(m.run.results)
		summary := lipgloss.NewStyle().Bold(true).Foreground(good)
		if observed < total {
			summary = summary.Foreground(bad)
		}
		b.WriteString(titleStyle.Render(m.run.sc.name) + "  " + summary.Render(fmt.Sprintf("%d/%d checks observed", observed, total)))
		if m.runErr != nil {
			b.WriteString(dimStyle.Render(fmt.Sprintf(" (%v)", m.runErr)))
		}
		b.WriteString("\n\n")
		for _, r := range m.run.results {
			s := entryStyles[entryPass]
			if !r.observed {
				s = entryStyles[entryFail]
			}
			b.WriteString(s.style.Width(width).Render(s.prefix+r.expect) + "\n")
		}
	case screenManual:
		b.WriteString(titleStyle.Render("Send events manually") + "\n\n")
		for i, item := range manualItems {
			b.WriteString(m.menuItem(i, item) + "\n")
		}
		b.WriteString("\n" + dimStyle.Width(width).Render("Lobby and gameover each also send a lobby phase. Player events collect the round's participants and roles for gameover; lobby and gameover reset them."))
	case screenForm:
		b.WriteString(m.form.View())
	}
	return b.String()
}

func (m *model) menuItem(i int, label string) string {
	if i == m.cursor {
		return titleStyle.Render("▸ " + label)
	}
	return "  " + label
}

func (m *model) panelView(width int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Sent game state") + "\n\n")
	if m.s == nil {
		b.WriteString(dimStyle.Render("Not connected"))
		return b.String()
	}
	v := m.s.view
	phaseName := "—"
	if v.phase != game.UNINITIALIZED {
		phaseName = string(v.phase.ToString())
	}
	b.WriteString(dimStyle.Render("Phase  ") + phaseName + "\n")
	if v.hasLobby {
		b.WriteString(dimStyle.Render("Lobby  ") + v.lobby.LobbyCode + "\n")
		b.WriteString(dimStyle.Render("Region ") + v.lobby.Region.ToString() + "\n")
		b.WriteString(dimStyle.Render("Map    ") + game.MapNames[v.lobby.PlayMap] + "\n")
	}
	b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Players (%d)", len(v.players))) + "\n")
	for _, p := range v.players {
		swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(playerColors[p.Color])).Render("●")
		name := p.Name
		var tags []string
		if p.IsDead {
			tags = append(tags, "dead")
			name = dimStyle.Strikethrough(true).Render(name)
		}
		if m.s.isImpostor(p.Name) {
			tags = append(tags, "impostor")
		}
		line := swatch + " " + name
		if len(tags) > 0 {
			line += " " + errStyle.Render(strings.Join(tags, " "))
		}
		b.WriteString(lipgloss.NewStyle().MaxWidth(width).Render(line) + "\n")
	}
	return b.String()
}
