package main

import (
	"bytes"
	"testing"

	"github.com/spyzhov/ajson"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	"google.golang.org/grpc/interop/grpc_testing"
)

type testMsg struct {
	line []byte
	err  error
}

type testMsgReader struct {
	lines []testMsg
	index int
}

func newTestMsgReader(lines []testMsg) MsgReader {
	return &testMsgReader{
		lines: lines,
		index: 0,
	}
}

func (mr *testMsgReader) ReadLine(names []string, opts ...ReadLineOpt) ([]byte, error) {
	res := mr.lines[mr.index]
	if res.err != nil {
		return nil, res.err
	}
	mr.index++
	return res.line, nil
}

func TestProtoCmdMsgBuffer(t *testing.T) {
	rl := newTestMsgReader([]testMsg{
		{[]byte("??"), nil},
		{nil, ErrInterruptTerm},
	})

	md := (*grpc_testing.SimpleRequest)(nil).ProtoReflect().Descriptor()

	result := &bytes.Buffer{}
	b := newMsgBuffer(&msgBufferOptions{
		reader:      rl,
		messageDesc: md,
		msgFormat:   caller.JSON,
		w:           result,
	})

	_, err := b.ReadMessage()
	if err != nil && err != ErrInterruptTerm {
		t.Fatal(err)
	}

	if !bytes.Contains(result.Bytes(), []byte("message SimpleRequest")) {
		t.Errorf("expected message SimpleRequest in the output, got %s", result.String())
	}
}

func TestHelpCmdMsgBuffer(t *testing.T) {
	rl := newTestMsgReader([]testMsg{
		{[]byte("?"), nil},
		{nil, ErrInterruptTerm},
	})

	md := (*grpc_testing.SimpleRequest)(nil).ProtoReflect().Descriptor()

	buf := &bytes.Buffer{}
	b := newMsgBuffer(&msgBufferOptions{
		reader:      rl,
		messageDesc: md,
		msgFormat:   caller.JSON,
		w:           buf,
	})

	_, err := b.ReadMessage()
	if err != nil && err != ErrInterruptTerm {
		t.Fatal(err)
	}

	res := buf.Bytes()
	root, err := ajson.Unmarshal(res)
	if err != nil {
		t.Fatalf("error unmarshaling result json: %v", err)
	}

	if jsonInt32(root, "$.response_size") != int32(0) {
		t.Errorf("response_size is invalid: %s", res)
	}
}
