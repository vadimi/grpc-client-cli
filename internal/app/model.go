package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Config configures the interactive application.
type Config struct {
	// Services is the list of services to choose from.
	Services caller.ServiceMetaList
	// Executor performs the grpc calls.
	Executor CallExecutor
	// Service optionally preselects a service by partial name.
	Service string
	// Method optionally preselects a method of the service by name.
	Method string
	// Deadline is the per call timeout in seconds.
	Deadline int
	// Verbose appends call statistics to the results.
	Verbose bool
	// InFormat is the request message format.
	InFormat caller.MsgFormat
	// OutFormat is the result message format.
	OutFormat caller.MsgFormat
	// Discover stops after the service selection and reports the chosen
	// service so its proto definition can be printed.
	Discover bool
}

// RunResult reports how the interactive session ended.
type RunResult struct {
	// Service is the selected service in discover mode.
	Service string
}

// Run starts the interactive application. It returns ErrInterrupted when
// the user quits with ctrl+c.
func Run(cfg *Config) (*RunResult, error) {
	// in discover mode a preselected service skips the ui entirely
	if cfg.Discover && cfg.Service != "" {
		if svc, err := ResolveService(cfg.Services, cfg.Service); err == nil {
			return &RunResult{Service: svc}, nil
		}
	}

	return runAppModel(newAppModel(cfg))
}

// runAppModel executes the application program. The program options can be
// replaced for testing.
func runAppModel(m *appModel, progOpts ...tea.ProgramOption) (*RunResult, error) {
	p := tea.NewProgram(m, progOpts...)

	final, err := p.Run()
	if err != nil {
		return nil, err
	}

	am, ok := final.(*appModel)
	if !ok {
		return nil, fmt.Errorf("unexpected final application model type %T", final)
	}

	if am.interrupted {
		return nil, ErrInterrupted
	}

	if am.runErr != nil {
		return nil, am.runErr
	}

	return &RunResult{Service: am.service}, nil
}

// appState is the screen the application is on.
type appState int

const (
	stateService appState = iota
	stateMethod
	stateEditor
	stateCalling
	stateResults
)

// callEvent is one event of a running call: a result message or the
// terminal status.
type callEvent struct {
	result []byte
	err    error
	done   bool
}

type callEventMsg callEvent

// callInfo tracks a call that is in flight.
type callInfo struct {
	ctx       context.Context
	cancel    context.CancelFunc
	events    chan callEvent
	streaming bool
}

// callEventBufferSize is the capacity of the call event channel. It is
// large enough that a call can report all of its events without a
// concurrent consumer, which keeps the event order intact when commands
// are executed strictly one after another (as in tests); a producer blocks
// beyond it, which simply applies backpressure.
const callEventBufferSize = 64

// appModel is the root model of the interactive application. It owns the
// screen state machine and wires the child components together.
type appModel struct {
	cfg    *Config
	theme  theme
	state  appState
	width  int
	height int

	services *listSelector
	methods  *listSelector
	editor   *msgEditor
	results  *resultsPanel
	spinner  spinner.Model

	service string
	svc     *caller.ServiceMeta
	method  protoreflect.MethodDescriptor

	call        *callInfo
	callStart   time.Time
	lastDur     time.Duration
	lastErr     error
	interrupted bool
	runErr      error
}

func newAppModel(cfg *Config) *appModel {
	m := &appModel{
		cfg:    cfg,
		theme:  newTheme(true),
		state:  stateService,
		width:  defaultListWidth,
		height: 24,
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Line),
			spinner.WithStyle(newTheme(true).question),
		),
		results: newResultsPanel(cfg.OutFormat),
	}

	m.services = newListSelector(SelectorOptions{
		Title:   "Choose a service:",
		Options: serviceOptions(cfg.Services),
	})
	m.setSize(m.width, m.height)

	// a matching preselection skips the corresponding screen, like the
	// service and method arguments did before
	if svc, err := ResolveService(cfg.Services, cfg.Service); err == nil {
		m.selectService(svc)
		if method, err := ResolveMethod(m.svc, cfg.Method); err == nil {
			m.selectMethod(method)
		}
	}

	return m
}

// Init implements tea.Model.
func (m *appModel) Init() tea.Cmd {
	// query the terminal background color to pick light or dark styles
	return tea.RequestBackgroundColor
}

// Update implements tea.Model.
func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.applyTheme(msg.IsDark())
		return m, nil

	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		if m.state != stateCalling {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case callEventMsg:
		return m.updateCall(callEvent(msg))

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.interrupted = true
			var cmds []tea.Cmd
			if m.call != nil {
				m.call.cancel()
			}
			cmds = append(cmds, tea.Quit)
			return m, tea.Batch(cmds...)
		}
		return m.updateKey(msg)
	}

	// remaining messages (paste events, cursor blinks, ...) go to the
	// active child component
	return m.updateChild(msg)
}

// updateKey routes a key press to the active screen.
func (m *appModel) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateService:
		return m.updateServiceSelect(msg)
	case stateMethod:
		return m.updateMethodSelect(msg)
	case stateEditor:
		return m.updateEditor(msg)
	case stateCalling:
		return m.updateCalling(msg)
	case stateResults:
		return m.updateResults(msg)
	}
	return m, nil
}

// updateChild routes a non key message to the active screen.
func (m *appModel) updateChild(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateService:
		// the list filters asynchronously and reports the matches in a
		// message
		return m, m.services.Update(msg)
	case stateMethod:
		return m, m.methods.Update(msg)
	case stateEditor:
		if m.editor != nil {
			_, _, cmd := m.editor.Update(msg)
			return m, cmd
		}
	case stateResults:
		cmd := m.results.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *appModel) updateServiceSelect(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	cmd := m.services.Update(msg)

	if msg.String() == "enter" {
		if choice, ok := m.services.Selected(); ok {
			if m.cfg.Discover {
				m.service = choice
				return m, tea.Quit
			}
			m.selectService(choice)
		}
	}

	return m, cmd
}

func (m *appModel) updateMethodSelect(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// the filter state is captured before the key is processed so that esc
	// first cancels an active filter and only goes back when the list is
	// not filtered
	filtered := m.methods.Filtering()

	cmd := m.methods.Update(msg)

	switch msg.String() {
	case "enter":
		if choice, ok := m.methods.Selected(); ok {
			switch choice {
			case goBackOption:
				m.state = stateService
			default:
				if method, err := ResolveMethod(m.svc, choice); err == nil {
					m.selectMethod(method)
				}
			}
		}
	case "esc":
		if !filtered {
			m.state = stateService
		}
	}

	return m, cmd
}

func (m *appModel) updateEditor(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	action, messages, cmd := m.editor.Update(msg)

	switch action {
	case editorBack:
		m.state = stateMethod
	case editorSend:
		return m, m.startCall(messages)
	}

	return m, cmd
}

func (m *appModel) updateCalling(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		// cancel the call; the terminal event finalizes the screen
		if m.call != nil {
			m.call.cancel()
		}
	}

	// no interaction while a call is in flight
	return m, nil
}

func (m *appModel) updateResults(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "ctrl+d":
		// adjust the previous message and send again
		m.editor.reset()
		m.state = stateEditor
		return m, nil
	case "esc":
		m.state = stateMethod
		return m, nil
	}

	// remaining keys scroll the results
	cmd := m.results.Update(msg)
	return m, cmd
}

// selectService records the chosen service and prepares the method list.
func (m *appModel) selectService(name string) {
	m.service = name
	m.svc = findService(m.cfg.Services, name)
	m.methods = newListSelector(SelectorOptions{
		Title:   "Choose a method:",
		Options: methodOptions(m.svc),
	})
	m.methods.applyTheme(m.theme)
	m.methods.SetSize(m.contentWidth(), m.contentHeight())
	m.state = stateMethod
}

// selectMethod records the chosen method and prepares the message editor.
func (m *appModel) selectMethod(method protoreflect.MethodDescriptor) {
	m.method = method
	desc := method.Input()
	m.editor = newMsgEditor(editorOptions{
		messageDesc: desc,
		format:      m.cfg.InFormat,
		fieldNames:  FieldNames(desc),
		streaming:   method.IsStreamingClient(),
		defaults:    MessageDefaults(desc),
		protoText:   MessageProto(desc),
	})
	m.editor.applyTheme(m.theme)
	m.editor.SetSize(m.contentWidth(), m.contentHeight())
	m.state = stateEditor
}

// startCall sends the request messages and starts listening for call
// events.
func (m *appModel) startCall(messages [][]byte) tea.Cmd {
	ctx, cancel := context.WithTimeout(rpc.WithStatsCtx(context.Background()), time.Duration(m.cfg.Deadline)*time.Second)
	events := make(chan callEvent, callEventBufferSize)
	m.call = &callInfo{
		ctx:       ctx,
		cancel:    cancel,
		events:    events,
		streaming: m.method.IsStreamingServer(),
	}
	m.callStart = time.Now()
	m.lastErr = nil
	m.results.reset()
	if m.call.streaming {
		m.results.beginStream()
	}
	m.state = stateCalling

	exec, method := m.cfg.Executor, m.method

	runCall := func() tea.Msg {
		if method.IsStreamingServer() {
			err := exec.CallStreaming(ctx, method, messages, func(r []byte) {
				events <- callEvent{result: r}
			})
			events <- callEvent{done: true, err: err}
		} else {
			result, err := exec.CallUnary(ctx, method, messages)
			if err == nil {
				events <- callEvent{result: result}
			}
			events <- callEvent{done: true, err: err}
		}
		return nil
	}

	return tea.Batch(runCall, m.waitEvent(), m.spinner.Tick)
}

// waitEvent returns a command that waits for the next call event.
func (m *appModel) waitEvent() tea.Cmd {
	events := m.call.events
	return func() tea.Msg {
		e, ok := <-events
		if !ok {
			return nil
		}
		return callEventMsg(e)
	}
}

// updateCall processes a call event.
func (m *appModel) updateCall(e callEvent) (tea.Model, tea.Cmd) {
	if m.call == nil {
		return m, nil
	}

	if !e.done {
		if e.result != nil {
			if m.call.streaming {
				m.results.addStreamResult(e.result)
			} else {
				m.results.addSingle(e.result)
			}
		}
		return m, m.waitEvent()
	}

	return m.finishCall(e.err)
}

// finishCall completes the call screen and shows the results.
func (m *appModel) finishCall(err error) (tea.Model, tea.Cmd) {
	ctx := m.call.ctx
	m.call.cancel()
	m.call = nil
	m.lastDur = time.Since(m.callStart)
	m.lastErr = err

	if m.method.IsStreamingServer() {
		m.results.endStream()
	}

	if err != nil {
		if !caller.IsErrTransient(err) {
			// parity with the previous behavior: errors that are not
			// transient end the application
			m.runErr = err
			return m, tea.Quit
		}
		m.results.appendText(m.theme.err.Render("Error: " + err.Error()))
	}

	if m.cfg.Verbose {
		stats := rpc.ExtractRpcStats(ctx)
		m.results.appendFn(func(w io.Writer) { PrintVerbose(w, stats, err) })
	}

	m.state = stateResults
	return m, nil
}

// setSize resizes the application and its child components.
func (m *appModel) setSize(width, height int) {
	m.width, m.height = width, height
	w, h := m.contentWidth(), m.contentHeight()
	m.services.SetSize(w, h)
	if m.methods != nil {
		m.methods.SetSize(w, h)
	}
	if m.editor != nil {
		m.editor.SetSize(w, h)
	}
	m.results.SetSize(w, h)
}

// frameHeight is the rows taken by the rules above and below the active
// screen.
const frameHeight = 2

// contentHeight is the height available to the active screen between the
// header and the footer.
func (m *appModel) contentHeight() int {
	return max(m.height-2-frameHeight, 1)
}

// contentWidth is the width available to the active screen.
func (m *appModel) contentWidth() int {
	return max(m.width, 1)
}

// applyTheme switches the application styles to a light or dark palette.
func (m *appModel) applyTheme(isDark bool) {
	m.theme = newTheme(isDark)
	m.spinner.Style = m.theme.question
	m.results.applyTheme(m.theme)
	m.services.applyTheme(m.theme)
	if m.methods != nil {
		m.methods.applyTheme(m.theme)
	}
	if m.editor != nil {
		m.editor.applyTheme(m.theme)
	}
}

// View implements tea.Model.
func (m *appModel) View() tea.View {
	// only horizontal rules frame the screen: side borders and padding
	// would end up in text copied from the terminal
	rule := m.theme.rule.Render(strings.Repeat("─", max(m.width, 1)))
	content := m.contentView()
	// pad with empty lines only, so no trailing spaces get copied either
	if n := strings.Count(content, "\n") + 1; n < m.contentHeight() {
		content += strings.Repeat("\n", m.contentHeight()-n)
	}
	return tea.NewView(strings.Join([]string{m.headerView(), rule, content, rule, m.footerView()}, "\n"))
}

// headerView renders the header bar: the app badge and the breadcrumb with
// the current selection on the left, the status badge on the right.
func (m *appModel) headerView() string {
	left := m.theme.badge.Render("grpc-client-cli")

	if m.service != "" {
		left += " " + m.theme.header.Render(m.service)
	}
	if m.method != nil && m.state != stateService && m.state != stateMethod {
		left += " " + m.theme.headerDim.Render("›") + " " + m.theme.header.Render(string(m.method.Name()))
	}

	status := m.statusView()
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(status)
	if gap < 1 {
		// not enough room for the status badge
		return ansi.Truncate(left, max(m.width, 1), "…")
	}

	return left + strings.Repeat(" ", gap) + status
}

// statusView renders the status badge of the current screen.
func (m *appModel) statusView() string {
	switch m.state {
	case stateService:
		return m.theme.statusIdle.Render("services")
	case stateMethod:
		return m.theme.statusIdle.Render("methods")
	case stateEditor:
		return m.theme.statusIdle.Render("request")
	case stateCalling:
		return m.theme.statusBusy.Render("calling…")
	default: // stateResults
		dur := m.lastDur.Round(time.Millisecond).String()
		if m.lastErr != nil {
			return m.theme.statusErr.Render("error " + dur)
		}
		return m.theme.statusOK.Render("OK " + dur)
	}
}

// contentView renders the active screen.
func (m *appModel) contentView() string {
	switch m.state {
	case stateService:
		return m.services.View()
	case stateMethod:
		return m.methods.View()
	case stateEditor:
		return m.editor.View()
	default: // stateCalling, stateResults
		return m.results.View()
	}
}

// footerView renders the key hints of the active screen.
func (m *appModel) footerView() string {
	var hints [][2]string
	switch m.state {
	case stateService:
		hints = [][2]string{{"↑/↓", "move"}, {"/", "filter"}, {"enter", "select"}}
	case stateMethod:
		hints = [][2]string{{"↑/↓", "move"}, {"/", "filter"}, {"enter", "select"}, {"esc", "back"}}
	case stateEditor:
		hints = [][2]string{{"ctrl+d", "send"}, {"tab", "complete"}, {"?", "defaults"}, {"??", "proto"}, {"esc", "back"}}
	case stateCalling:
		return m.spinner.View() + " " + m.theme.keyDesc.Render("calling "+m.methodPath()+" • ") +
			m.theme.key.Render("esc") + " " + m.theme.keyDesc.Render("cancel")
	case stateResults:
		hints = [][2]string{{"enter", "send another"}, {"esc", "choose method"}, {"↑/↓", "scroll"}}
	}

	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = m.theme.key.Render(h[0]) + " " + m.theme.keyDesc.Render(h[1])
	}
	return strings.Join(parts, m.theme.keyDesc.Render("  •  "))
}

func (m *appModel) methodPath() string {
	if m.method == nil {
		return ""
	}
	svc := m.method.Parent().(protoreflect.ServiceDescriptor)
	return fmt.Sprintf("/%s/%s", svc.FullName(), m.method.Name())
}
