package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// editorAction reports what the editor wants the application to do next.
type editorAction int

const (
	editorNone editorAction = iota
	// editorBack returns to method selection.
	editorBack
	// editorSend starts a call with the collected request messages.
	editorSend
)

// editorOptions configures a message editor for a method input.
type editorOptions struct {
	messageDesc protoreflect.MessageDescriptor
	format      caller.MsgFormat
	fieldNames  []string
	streaming   bool // the method accepts multiple request messages
	defaults    string
	protoText   string
}

// msgEditor edits request messages. It provides field name completion,
// validation, message defaults and request queueing for client streaming
// methods, mirroring the abilities of the previous line based editor.
type msgEditor struct {
	textarea      textarea.Model
	theme         theme
	format        caller.MsgFormat
	messageDesc   protoreflect.MessageDescriptor
	fieldNames    []string
	streaming     bool
	defaults      string
	protoText     string
	queue         [][]byte
	suggestions   []string
	notice        string
	noticeErr     bool
	preview       string
	width         int
	contentHeight int
}

func newMsgEditor(opts editorOptions) *msgEditor {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = opts.defaults

	e := &msgEditor{
		textarea:    ta,
		theme:       newTheme(true),
		format:      opts.format,
		messageDesc: opts.messageDesc,
		fieldNames:  opts.fieldNames,
		streaming:   opts.streaming,
		defaults:    opts.defaults,
		protoText:   opts.protoText,
		width:       defaultListWidth,
	}

	e.textarea.SetStyles(e.theme.textAreaStyles())
	e.textarea.Focus()
	e.contentHeight = 10
	e.SetSize(e.width, e.contentHeight)

	return e
}

// Update processes a message. When the editor completes a request it
// returns editorSend with the request messages; when the user leaves the
// editor it returns editorBack.
func (e *msgEditor) Update(msg tea.Msg) (editorAction, [][]byte, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		e.textarea, cmd = e.textarea.Update(msg)
		return editorNone, nil, cmd
	}

	switch key.String() {
	case "esc":
		return editorBack, nil, nil

	case "tab":
		e.complete()
		return editorNone, nil, nil

	case "ctrl+d":
		// ctrl+d ends the message input like it ended the prompt line
		// before.
		return e.submit()
	}

	var cmd tea.Cmd
	e.textarea, cmd = e.textarea.Update(key)
	e.refreshPreview()
	return editorNone, nil, cmd
}

// View renders the editor area with a fixed height: the message title, the
// text area and one line each for suggestions, notices and the preview.
func (e *msgEditor) View() string {
	sections := []string{
		e.theme.question.Render("?") + " " + e.theme.title.Render(e.title()),
		e.textarea.View(),
		e.suggestionsView(),
		e.noticeView(),
		e.previewView(),
	}
	return strings.Join(sections, "\n")
}

func (e *msgEditor) title() string {
	return fmt.Sprintf("Message %s:", e.format)
}

// SetSize resizes the editor to the given content area.
func (e *msgEditor) SetSize(width, height int) {
	e.width = width
	e.contentHeight = height

	// title, suggestions, notice and preview take one row each
	taHeight := height - 4
	if taHeight < 3 {
		taHeight = 3
	}
	e.textarea.SetWidth(width)
	e.textarea.SetHeight(taHeight)
}

// applyTheme switches the text area styles to a light or dark palette.
func (e *msgEditor) applyTheme(t theme) {
	e.theme = t
	e.textarea.SetStyles(t.textAreaStyles())
}

// reset clears the request queue and notices, keeping the edited message
// so it can be adjusted and sent again.
func (e *msgEditor) reset() {
	e.queue = nil
	e.setNotice("", false)
	e.suggestions = nil
}

// submit handles ctrl+d: it validates and queues or sends the message
// under the cursor.
func (e *msgEditor) submit() (editorAction, [][]byte, tea.Cmd) {
	content := strings.TrimSpace(e.textarea.Value())

	switch content {
	case "?", "??", "proto":
		// preview commands, never a message
		return editorNone, nil, nil
	}

	if content == "" {
		if e.streaming && len(e.queue) > 0 {
			messages := e.queue
			e.queue = nil
			return editorSend, messages, nil
		}

		// ctrl+d on an empty editor goes back to method selection, like
		// ctrl+d on an empty prompt line before.
		return editorBack, nil, nil
	}

	if err := ValidateMessage([]byte(content), e.messageDesc, e.format); err != nil {
		e.setNotice(err.Error(), true)
		return editorNone, nil, nil
	}

	if e.streaming {
		e.queue = append(e.queue, []byte(content))
		e.textarea.SetValue("")
		e.refreshPreview()
		e.setNotice(fmt.Sprintf("%d message(s) queued, ctrl+d on an empty editor sends the request", len(e.queue)), false)
		return editorNone, nil, nil
	}

	return editorSend, [][]byte{[]byte(content)}, nil
}

// complete replaces the word before the cursor with the first matching
// field name and lists all matches.
func (e *msgEditor) complete() {
	word, ok := e.currentWord()
	if !ok {
		e.suggestions = nil
		return
	}

	e.suggestions = e.matchFields(word)
	if len(e.suggestions) == 0 {
		return
	}

	completion := e.suggestions[0]
	lines := strings.Split(e.textarea.Value(), "\n")
	row := e.textarea.Line()
	col := e.textarea.Column()
	if row < 0 || row >= len(lines) {
		return
	}

	line := lines[row]
	start := col - len(word)
	if start < 0 || start > len(line) || col > len(line) {
		return
	}

	lines[row] = line[:start] + completion + line[col:]
	e.textarea.SetValue(strings.Join(lines, "\n"))

	// SetValue leaves the cursor at the end of the input; move it right
	// after the completed word.
	e.textarea.CursorStart()
	for i := 0; i < row; i++ {
		e.textarea, _ = e.textarea.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	e.textarea.SetCursorColumn(start + len(completion))
}

// currentWord returns the word before the cursor. Word delimiters are
// json and text format separators.
func (e *msgEditor) currentWord() (string, bool) {
	lines := strings.Split(e.textarea.Value(), "\n")
	row := e.textarea.Line()
	col := e.textarea.Column()
	if row < 0 || row >= len(lines) {
		return "", false
	}

	line := lines[row]
	if col > len(line) {
		col = len(line)
	}

	start := col
	for start > 0 && !isWordDelimiter(rune(line[start-1])) {
		start--
	}

	if start == col {
		return "", false
	}

	return line[start:col], true
}

func isWordDelimiter(r rune) bool {
	switch r {
	case ' ', '\t', '"', '\'', ':', ',', '{', '}', '[', ']', '(', ')', '=', ';':
		return true
	}
	return false
}

// matchFields returns the field names that start with word,
// case-insensitively.
func (e *msgEditor) matchFields(word string) []string {
	var matches []string
	for _, name := range e.fieldNames {
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(word)) {
			matches = append(matches, name)
		}
	}
	return matches
}

// refreshPreview shows the message defaults or proto for the ? and ??
// preview commands.
func (e *msgEditor) refreshPreview() {
	e.preview = ""
	switch strings.TrimSpace(e.textarea.Value()) {
	case "?":
		e.preview = e.defaults
	case "??", "proto":
		e.preview = e.protoText
	}
}

func (e *msgEditor) setNotice(notice string, isErr bool) {
	e.notice = notice
	e.noticeErr = isErr
}

func (e *msgEditor) suggestionsView() string {
	if len(e.suggestions) == 0 {
		return ""
	}
	return e.theme.footer.Render("matches: " + strings.Join(e.suggestions, "  "))
}

func (e *msgEditor) noticeView() string {
	if e.notice == "" {
		return ""
	}
	if e.noticeErr {
		return e.theme.err.Render(e.notice)
	}
	return e.theme.notice.Render(e.notice)
}

func (e *msgEditor) previewView() string {
	if e.preview == "" {
		return ""
	}
	return e.theme.footer.Render(ansi.Truncate(e.preview, max(e.width-1, 1), "…"))
}
