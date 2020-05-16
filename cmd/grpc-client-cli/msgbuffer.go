package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

type msgBuffer struct {
	opts *msgBufferOptions
}

type msgBufferOptions struct {
	prompt     string
	reader     *msgReader
	fieldNames []string
	helpText   string
}

func newMsgBuffer(opts *msgBufferOptions) *msgBuffer {
	return &msgBuffer{
		opts: opts,
	}
}

func (b *msgBuffer) ReadMessage(opts ...ReadLineOpt) ([]byte, error) {
	for {
		message, err := b.opts.reader.ReadLine(b.opts.fieldNames, opts...)
		if err != nil {
			if err == terminal.InterruptErr {
				return nil, terminal.InterruptErr
			}
			return message, err
		}

		normMsg := bytes.TrimSpace(message)
		if len(normMsg) > 0 {
			if bytes.Equal(normMsg, []byte("?")) {
				fmt.Println(b.opts.helpText)
				continue
			}
			return normMsg, nil
		}
	}
}

func (b *msgBuffer) ReadMessages() ([][]byte, error) {
	if b.opts == nil || b.opts.reader == nil {
		return nil, errors.New("no msg reader is configured")
	}

	msg, err := b.ReadMessage()
	if err != nil {
		return nil, err
	}

	buf := [][]byte{msg}

	for {
		msg, err := b.ReadMessage(WithReadLinePrompt(b.opts.prompt))
		if err == terminal.InterruptErr {
			return nil, terminal.InterruptErr
		}

		// Ctrl+D will trigger io.EOF if the line is empty
		// it means no new messages are expected
		if err == io.EOF {
			return buf, nil
		}

		if err != nil {
			return nil, err
		}

		buf = append(buf, msg)
	}
}
