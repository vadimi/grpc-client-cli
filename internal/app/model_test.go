package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// fakeExecutor is a CallExecutor used to drive the application model
// without a grpc server.
type fakeExecutor struct {
	unaryResult []byte
	unaryErr    error
	streamErr   error
	streamFn    func(ctx context.Context, onResult func([]byte))
}

func (f *fakeExecutor) CallUnary(_ context.Context, _ protoreflect.MethodDescriptor, _ [][]byte) ([]byte, error) {
	return f.unaryResult, f.unaryErr
}

func (f *fakeExecutor) CallStreaming(ctx context.Context, _ protoreflect.MethodDescriptor, _ [][]byte, onResult func([]byte)) error {
	if f.streamFn != nil {
		f.streamFn(ctx, onResult)
		return f.streamErr
	}
	return f.streamErr
}

// transientError mimics the caller errors that let the user retry a call.
type transientError struct{}

func (transientError) Error() string   { return "transient error" }
func (transientError) Temporary() bool { return true }

func newTestConfig(exec CallExecutor) *Config {
	return &Config{
		Services:  testServiceList(),
		Executor:  exec,
		Deadline:  5,
		InFormat:  caller.JSON,
		OutFormat: caller.JSON,
	}
}

// sendMsg delivers a message to the model, executes the returned commands
// and feeds their messages back, skipping spinner ticks.
func sendMsg(t *testing.T, m *appModel, msg tea.Msg) {
	t.Helper()

	model, cmd := m.Update(msg)
	next, ok := model.(*appModel)
	require.True(t, ok)
	require.Same(t, m, next)
	execCmd(t, m, cmd, 0)
}

func execCmd(t *testing.T, m *appModel, cmd tea.Cmd, depth int) {
	t.Helper()
	if cmd == nil || depth > 8 {
		return
	}

	switch msg := cmd().(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, c := range msg {
			execCmd(t, m, c, depth+1)
		}
	case spinner.TickMsg, cursor.BlinkMsg:
		// animation messages drive the spinner and the cursor blink; they
		// are not needed in tests
	default:
		sendMsg(t, m, msg)
	}
}

func selectMethodInUI(t *testing.T, m *appModel) {
	t.Helper()

	// choose grpc_client_cli.testing.TestService, the fourth service in
	// the sorted list
	sendMsg(t, m, downKey)
	sendMsg(t, m, downKey)
	sendMsg(t, m, downKey)
	sendMsg(t, m, enterKey)
	require.Equal(t, stateMethod, m.state)
	require.Equal(t, "grpc_client_cli.testing.TestService", m.service)

	// move past the [..] entry and choose a method
	sendMsg(t, m, downKey)
	sendMsg(t, m, enterKey)
	require.Equal(t, stateEditor, m.state)
	require.NotNil(t, m.method)
}

// typeMessage types a request message into the editor.
func typeMessage(t *testing.T, m *appModel, msg string) {
	t.Helper()
	for _, r := range msg {
		sendMsg(t, m, runeKey(string(r)))
	}
}

func TestAppModelPreselection(t *testing.T) {
	cfg := newTestConfig(&fakeExecutor{})
	cfg.Service = "testservice"
	cfg.Method = "UnaryCall"

	m := newAppModel(cfg)

	assert.Equal(t, stateEditor, m.state, "a matching service and method skip their screens")
	assert.Equal(t, "grpc_client_cli.testing.TestService", m.service)
	assert.Equal(t, "UnaryCall", string(m.method.Name()))
}

func TestAppModelPreselectionServiceOnly(t *testing.T) {
	cfg := newTestConfig(&fakeExecutor{})
	cfg.Service = "testservice"

	m := newAppModel(cfg)

	assert.Equal(t, stateMethod, m.state, "only the service screen is skipped")
}

func TestAppModelPreselectionNoMatchShowsService(t *testing.T) {
	cfg := newTestConfig(&fakeExecutor{})
	cfg.Service = "missing"

	m := newAppModel(cfg)

	assert.Equal(t, stateService, m.state, "an unknown service falls back to the selection screen")
}

func TestAppModelMethodBackNavigation(t *testing.T) {
	m := newAppModel(newTestConfig(&fakeExecutor{}))

	// esc at the service screen does nothing
	sendMsg(t, m, escKey)
	assert.Equal(t, stateService, m.state)

	sendMsg(t, m, enterKey)
	assert.Equal(t, stateMethod, m.state)

	// esc at the method screen goes back to the service screen
	sendMsg(t, m, escKey)
	assert.Equal(t, stateService, m.state)

	// the [..] entry goes back as well
	sendMsg(t, m, enterKey)
	assert.Equal(t, stateMethod, m.state)
	sendMsg(t, m, enterKey)
	assert.Equal(t, stateService, m.state, "the [..] entry returns to the service screen")
}

func TestAppModelEditorBackNavigation(t *testing.T) {
	cfg := newTestConfig(&fakeExecutor{})
	cfg.Service = "testservice"
	m := newAppModel(cfg)
	require.Equal(t, stateMethod, m.state)

	// esc at the editor goes back to the method screen
	sendMsg(t, m, downKey)
	sendMsg(t, m, enterKey)
	assert.Equal(t, stateEditor, m.state)
	sendMsg(t, m, escKey)
	assert.Equal(t, stateMethod, m.state)

	// ctrl+d on an empty editor goes back to the method screen as well
	sendMsg(t, m, downKey)
	sendMsg(t, m, enterKey)
	assert.Equal(t, stateEditor, m.state)
	sendMsg(t, m, ctrlDKey)
	assert.Equal(t, stateMethod, m.state)
}

func TestAppModelUnaryCallFlow(t *testing.T) {
	exec := &fakeExecutor{unaryResult: []byte(`{"ok": true}`)}
	m := newAppModel(newTestConfig(exec))

	selectMethodInUI(t, m)

	typeMessage(t, m, `{}`)
	sendMsg(t, m, ctrlDKey)

	assert.Equal(t, stateResults, m.state)
	assert.Contains(t, m.results.buf.String(), `"ok": true`)

	// enter goes back to the editor with the previous message
	sendMsg(t, m, enterKey)
	assert.Equal(t, stateEditor, m.state)
	assert.Equal(t, `{}`, m.editor.textarea.Value())
}

func TestAppModelStreamingCallFlow(t *testing.T) {
	exec := &fakeExecutor{}
	exec.streamFn = func(_ context.Context, onResult func([]byte)) {
		onResult([]byte(`{"n": 1}`))
		onResult([]byte(`{"n": 2}`))
	}
	cfg := newTestConfig(exec)
	cfg.Service = "testservice"
	cfg.Method = "StreamingOutputCall"

	m := newAppModel(cfg)
	assert.Equal(t, stateEditor, m.state)

	typeMessage(t, m, `{}`)
	sendMsg(t, m, ctrlDKey)

	assert.Equal(t, stateResults, m.state)
	assert.Contains(t, m.results.buf.String(), `[`)
	assert.Contains(t, m.results.buf.String(), `{"n": 1}`)
	assert.Contains(t, m.results.buf.String(), `{"n": 2}`)
	assert.Contains(t, m.results.buf.String(), `]`)
}

func TestAppModelEmptyStreamRendersEmptyArray(t *testing.T) {
	exec := &fakeExecutor{}
	cfg := newTestConfig(exec)
	cfg.Service = "testservice"
	cfg.Method = "StreamingOutputCall"

	m := newAppModel(cfg)
	sendMsg(t, m, runeKey("{"))
	sendMsg(t, m, runeKey("}"))
	sendMsg(t, m, ctrlDKey)

	assert.Equal(t, stateResults, m.state)
	assert.Contains(t, m.results.buf.String(), "[]", "a stream without results renders an empty array")
}

func TestAppModelTransientErrorShowsResults(t *testing.T) {
	exec := &fakeExecutor{unaryErr: transientError{}}
	cfg := newTestConfig(exec)
	cfg.Service = "testservice"
	cfg.Method = "UnaryCall"

	m := newAppModel(cfg)
	typeMessage(t, m, `{}`)
	sendMsg(t, m, ctrlDKey)

	assert.Equal(t, stateResults, m.state, "a transient error lets the user retry")
	assert.Contains(t, m.results.buf.String(), "Error: transient error")
	assert.NoError(t, m.runErr)
}

func TestAppModelFatalErrorQuits(t *testing.T) {
	exec := &fakeExecutor{unaryErr: errors.New("boom")}
	cfg := newTestConfig(exec)
	cfg.Service = "testservice"
	cfg.Method = "UnaryCall"

	m := newAppModel(cfg)
	typeMessage(t, m, `{}`)
	sendMsg(t, m, ctrlDKey)

	assert.False(t, m.interrupted)
	require.Error(t, m.runErr, "a non transient error ends the application")
}

func TestAppModelCtrlCInterrupts(t *testing.T) {
	m := newAppModel(newTestConfig(&fakeExecutor{}))

	model, cmd := m.Update(ctrlCKey)
	assert.Same(t, m, model.(*appModel))
	assert.True(t, m.interrupted)
	require.NotNil(t, cmd)
}

func TestAppModelCallingEscapeCancels(t *testing.T) {
	exec := &fakeExecutor{}
	exec.streamFn = func(ctx context.Context, _ func([]byte)) {
		<-ctx.Done()
	}
	cfg := newTestConfig(exec)
	cfg.Service = "testservice"
	cfg.Method = "StreamingOutputCall"

	m := newAppModel(cfg)

	// start the call but do not execute its command, it blocks until the
	// call context is cancelled
	typeMessage(t, m, `{}`)
	model, cmd := m.Update(ctrlDKey)
	require.Same(t, m, model.(*appModel))
	require.NotNil(t, cmd)
	assert.Equal(t, stateCalling, m.state)
	require.NotNil(t, m.call)

	// esc cancels the call
	sendMsg(t, m, escKey)
	select {
	case <-m.call.ctx.Done():
	default:
		t.Fatal("esc should cancel the call context")
	}

	// the terminal event the call goroutine delivers after cancellation;
	// the executor reports cancellation as a transient error, like every
	// call error except unavailable
	sendMsg(t, m, callEventMsg(callEvent{done: true, err: transientError{}}))

	assert.Equal(t, stateResults, m.state)
	assert.Contains(t, m.results.buf.String(), "Error:")
}

func TestAppModelDiscoverSelect(t *testing.T) {
	cfg := newTestConfig(&fakeExecutor{})
	cfg.Discover = true

	m := newAppModel(cfg)

	// choose grpc_client_cli.testing.TestService, the fourth service in
	// the sorted list
	sendMsg(t, m, downKey)
	sendMsg(t, m, downKey)
	sendMsg(t, m, downKey)
	sendMsg(t, m, enterKey)

	assert.Equal(t, "grpc_client_cli.testing.TestService", m.service)
}

func TestRunDiscoverPreselected(t *testing.T) {
	cfg := newTestConfig(&fakeExecutor{})
	cfg.Discover = true
	cfg.Service = "testservice"

	res, err := Run(cfg)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "grpc_client_cli.testing.TestService", res.Service)
}

func TestRunProgramInterrupted(t *testing.T) {
	in := strings.NewReader("\x03") // ctrl+c
	var out bytes.Buffer

	_, err := runAppModel(newAppModel(newTestConfig(&fakeExecutor{})),
		tea.WithInput(in),
		tea.WithOutput(&out),
		tea.WithWindowSize(100, 30),
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInterrupted)
}

func TestRunProgramUnaryFlow(t *testing.T) {
	exec := &fakeExecutor{unaryResult: []byte(`{"ok": true}`)}
	cfg := newTestConfig(exec)

	// the keys are written with pauses like a user would type them, so
	// the call can finish before the session is interrupted
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		io.WriteString(pw, "\x1b[B\x1b[B\x1b[B\r") // choose TestService
		io.WriteString(pw, "\x1b[B\r")             // skip [..], choose a method
		time.Sleep(100 * time.Millisecond)
		io.WriteString(pw, "{}")   // the message
		io.WriteString(pw, "\x04") // ctrl+d sends it
		time.Sleep(300 * time.Millisecond)
		io.WriteString(pw, "\x03") // ctrl+c quits
	}()

	var out bytes.Buffer

	res, err := runAppModel(newAppModel(cfg),
		tea.WithInput(pr),
		tea.WithOutput(&out),
		tea.WithWindowSize(100, 30),
	)

	// the session ends with ctrl+c
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInterrupted)
	assert.Nil(t, res)

	assert.Contains(t, out.String(), `"ok": true`, "the call result is rendered")
}

func TestAppModelServiceFilterAppliesMatches(t *testing.T) {
	m := newAppModel(newTestConfig(&fakeExecutor{}))
	m.setSize(60, 14)

	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	var cmd tea.Cmd
	for _, r := range "bbb" {
		_, cmd = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// the list reports the filter matches asynchronously; they must reach
	// it. Other commands, like the cursor blink, wait and are skipped.
	var cmds []tea.Cmd
	if batch, ok := cmd().(tea.BatchMsg); ok {
		cmds = batch
	} else {
		cmds = []tea.Cmd{cmd}
	}
	for _, c := range cmds {
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		select {
		case msg := <-done:
			m.Update(msg)
		case <-time.After(100 * time.Millisecond):
		}
	}

	view := ansi.Strip(m.View().Content)
	assert.Contains(t, view, "bbb.SecondService")
	assert.NotContains(t, view, "aaa.FirstService")
}
