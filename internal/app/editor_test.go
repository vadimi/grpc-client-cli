package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	clitesting "github.com/vadimi/grpc-client-cli/internal/testing/grpc_testing"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func testMessageDescriptor() protoreflect.MessageDescriptor {
	md := clitesting.File_test_proto.Messages().ByName("SimpleRequest")
	if md == nil {
		panic("SimpleRequest message not found in test proto")
	}
	return md
}

func newTestEditor(streaming bool) *msgEditor {
	md := testMessageDescriptor()
	return newMsgEditor(editorOptions{
		messageDesc: md,
		format:      caller.JSON,
		fieldNames:  FieldNames(md),
		streaming:   streaming,
		defaults:    MessageDefaults(md),
		protoText:   MessageProto(md),
	})
}

// typeText sends each character of s as a key press.
func typeText(e *msgEditor, s string) {
	for _, r := range s {
		e.Update(runeKey(string(r)))
	}
}

func TestEditorSubmitUnary(t *testing.T) {
	e := newTestEditor(false)

	typeText(e, `{"user": {"id": 1}}`)

	action, messages, _ := e.Update(ctrlDKey)
	assert.Equal(t, editorSend, action)
	require.Len(t, messages, 1)
	assert.Equal(t, `{"user": {"id": 1}}`, string(messages[0]))
}

func TestEditorSubmitInvalidMessage(t *testing.T) {
	e := newTestEditor(false)

	typeText(e, `{"user": }`)
	action, messages, _ := e.Update(ctrlDKey)

	assert.Equal(t, editorNone, action)
	assert.Nil(t, messages)
	assert.NotEmpty(t, e.notice, "a validation error should be shown")
	assert.True(t, e.noticeErr)
	assert.Contains(t, e.View(), e.notice)
}

func TestEditorSubmitEmptyGoesBack(t *testing.T) {
	e := newTestEditor(false)

	action, _, _ := e.Update(ctrlDKey)
	assert.Equal(t, editorBack, action)
}

func TestEditorStreamingQueue(t *testing.T) {
	e := newTestEditor(true)

	typeText(e, `{"user": {"id": 1}}`)
	action, messages, _ := e.Update(ctrlDKey)
	assert.Equal(t, editorNone, action, "the first message is queued")
	assert.Nil(t, messages)
	assert.Len(t, e.queue, 1)
	assert.Empty(t, e.textarea.Value(), "the editor is cleared for the next message")

	typeText(e, `{"user": {"id": 2}}`)
	e.Update(ctrlDKey)
	assert.Len(t, e.queue, 2)

	// ctrl+d on an empty editor sends the queue
	action, messages, _ = e.Update(ctrlDKey)
	assert.Equal(t, editorSend, action)
	require.Len(t, messages, 2)
	assert.Equal(t, `{"user": {"id": 1}}`, string(messages[0]))
	assert.Equal(t, `{"user": {"id": 2}}`, string(messages[1]))
}

func TestEditorEscapeGoesBack(t *testing.T) {
	e := newTestEditor(false)

	action, _, _ := e.Update(escKey)
	assert.Equal(t, editorBack, action)
}

func TestEditorPreviewCommands(t *testing.T) {
	// EchoStatus has scalar fields, so its defaults are visible in the
	// preview
	md := clitesting.File_test_proto.Messages().ByName("EchoStatus")
	require.NotNil(t, md)
	e := newMsgEditor(editorOptions{
		messageDesc: md,
		format:      caller.JSON,
		fieldNames:  FieldNames(md),
		defaults:    MessageDefaults(md),
		protoText:   MessageProto(md),
	})

	// ? shows the message defaults
	typeText(e, "?")
	assert.Contains(t, e.preview, `"code"`)
	assert.Contains(t, e.View(), `"code"`)

	// ctrl+d on a preview command never sends
	action, messages, _ := e.Update(ctrlDKey)
	assert.Equal(t, editorNone, action)
	assert.Nil(t, messages)

	// ?? shows the message proto
	e.textarea.SetValue("")
	typeText(e, "??")
	assert.Contains(t, e.preview, "message EchoStatus")

	// editing a real message clears the preview
	e.textarea.SetValue("")
	typeText(e, "{}")
	assert.Empty(t, e.preview)
}

func TestEditorCompletion(t *testing.T) {
	e := newTestEditor(false)

	// type an opening quote and a field prefix
	typeText(e, `"nam`)

	action, _, _ := e.Update(tabKey)
	assert.Equal(t, editorNone, action)
	assert.Equal(t, `"name`, e.textarea.Value(), "the word before the cursor is completed")
	assert.Equal(t, []string{"name"}, e.suggestions)
	assert.Contains(t, e.View(), "matches: name")
}

func TestEditorCompletionNoMatch(t *testing.T) {
	e := newTestEditor(false)

	typeText(e, `"zzz`)

	e.Update(tabKey)
	assert.Equal(t, `"zzz`, e.textarea.Value())
	assert.Empty(t, e.suggestions)
}

func TestEditorResetKeepsMessage(t *testing.T) {
	e := newTestEditor(false)

	typeText(e, `{"user": {"id": 1}}`)
	e.setNotice("queued", false)
	e.suggestions = []string{"user"}

	e.reset()

	assert.Empty(t, e.notice)
	assert.Empty(t, e.suggestions)
	assert.Equal(t, `{"user": {"id": 1}}`, e.textarea.Value(), "the last message is kept for another send")
}

func TestEditorViewStableHeight(t *testing.T) {
	e := newTestEditor(false)
	e.SetSize(80, 20)

	lines := strings.Split(e.View(), "\n")
	assert.Len(t, lines, 20, "title + textarea + 3 status lines fill the content area")
}
