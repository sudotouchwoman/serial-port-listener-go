package connection

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
)

type StringBuffer struct {
	*strings.Reader
	*strings.Builder
}

func (sb *StringBuffer) Read(p []byte) (n int, err error) {
	return sb.Reader.Read(p)
}

func TestSerialConnection_Listen(t *testing.T) {
	const sourceString = "a serial connector test"
	tokens := strings.Split(sourceString, " ")

	type fields struct {
		ReadWriter      io.ReadWriter
		Tokenizer       bufio.SplitFunc
		ExpectedRead    []string
		ExpectedWritten string
		CancelCtx       bool
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
		{
			name: "Listening for updates from string",
			fields: fields{
				ReadWriter: &StringBuffer{
					Reader:  strings.NewReader(sourceString),
					Builder: &strings.Builder{},
				},
				Tokenizer:       bufio.ScanWords,
				ExpectedRead:    tokens,
				ExpectedWritten: sourceString,
			},
		}, {

			name: "Listening for updates with context timeout",
			fields: fields{
				ReadWriter: &StringBuffer{
					Reader:  strings.NewReader(sourceString),
					Builder: &strings.Builder{},
				},
				Tokenizer:       bufio.ScanWords,
				ExpectedRead:    tokens,
				ExpectedWritten: sourceString,
				CancelCtx:       true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.fields.CancelCtx {
				cancel()
			}
			ss := &SerialConnection{
				ReadWriter: tt.fields.ReadWriter,
				Context:    ctx,
				Tokenizer:  tt.fields.Tokenizer,
				DataChan:   make(chan []byte, 1),
				errChan:    make(chan error, 1),
			}
			go ss.Listen()
			for word := range ss.DataChan {
				wordStr, ExpectedWord := string(word), tt.fields.ExpectedRead[i]
				i++
				if wordStr == ExpectedWord {
					continue
				}
				t.Errorf("reading from serial error. want = %v, got= %v", wordStr, ExpectedWord)
			}
			if err := <-ss.errChan; err != nil {
				t.Errorf(err.Error())
			}
			if _, open := <-ss.errChan; open {
				t.Error("expected SerialConnection.errChan to be closed by now")
			}
		})
	}
}
