package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/peterh/liner"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

type msgReaderSettings struct {
	HistoryFile string
	Prompt      string
}

type MsgReader interface {
	ReadLine(names []string, opts ...ReadLineOpt) ([]byte, error)
}

type msgReader struct {
	settings *msgReaderSettings
	line     *liner.State
}

type readLineOptions struct {
	prompt string
}

type ReadLineOpt func(*readLineOptions)

func WithReadLinePrompt(p string) ReadLineOpt {
	return func(o *readLineOptions) {
		o.prompt = p
	}
}

const readBufferSize = 8388608

func newMsgReader(settings *msgReaderSettings) (*msgReader, error) {
	r := &msgReader{
		settings: settings,
		line:     liner.NewLiner(),
	}

	r.line.SetCtrlCAborts(true)
	r.line.SetBeep(false)

	if r.settings.HistoryFile != "" {
		if err := r.readHistory(); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *msgReader) Close() (err error) {
	defer func() {
		err = r.line.Close()
	}()
	return
}

func (r *msgReader) ReadLine(names []string, opts ...ReadLineOpt) ([]byte, error) {
	r.line.SetWordCompleter(func(line string, pos int) (head string, completions []string, tail string) {
		h := ""
		word := line[0:pos]
		qi := strings.LastIndex(word, `"`)
		if qi >= 0 {
			h = line[0 : qi+1]
			word = line[qi+1 : pos]
		}
		for _, n := range names {
			if strings.HasPrefix(n, strings.ToLower(word)) {
				head = h
				completions = append(completions, n)
				tail = line[pos:]
				return
			}
		}
		return
	})

	rlOpts := &readLineOptions{}
	for _, o := range opts {
		o(rlOpts)
	}

	prompt := r.settings.Prompt
	if rlOpts.prompt != "" {
		prompt = rlOpts.prompt
	}

	msg, err := r.line.Prompt(prompt)
	if err != nil {
		if err == liner.ErrPromptAborted {
			return nil, terminal.InterruptErr
		}
		return nil, err
	}

	r.line.AppendHistory(msg)
	err = r.writeHistory()
	if err != nil {
		return nil, err
	}

	return []byte(msg), err
}

func (r *msgReader) readHistory() (err error) {
	f, err := os.Open(r.settings.HistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_, err = r.readHistorySize(f, readBufferSize)
	if err != nil {
		log.Println("unable to read cmd history.", err)
	}
	defer func() {
		err = f.Close()
	}()
	return nil
}

func (r *msgReader) writeHistory() (err error) {
	f, err := os.Create(r.settings.HistoryFile)
	if err != nil {
		return err
	}
	_, err = r.line.WriteHistory(f)
	if err != nil {
		return err
	}
	defer func() {
		err = f.Close()
	}()
	return nil
}

// From https://github.com/peterh/liner/blob/master/common.go#L80
func (r *msgReader) readHistorySize(reader io.Reader, bufferSize int) (num int, err error) {
	in := bufio.NewReaderSize(reader, bufferSize)
	num = 0
	for {
		line, part, err := in.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return num, err
		}
		if part {
			return num, fmt.Errorf("line %d is too long", num+1)
		}
		if !utf8.Valid(line) {
			return num, fmt.Errorf("invalid string at line %d", num+1)
		}
		num++
		r.line.AppendHistory(string(line))
	}
	return num, nil
}
