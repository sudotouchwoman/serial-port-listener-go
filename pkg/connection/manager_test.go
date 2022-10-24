package connection

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
)

const (
	prohibited = "connection name is banned"
)

var ErrProhibitedName = errors.New(prohibited)

func GetMockProvider(names map[string]int) ConnectionProvider {
	return func(s string) (wr io.ReadWriter, cancel func(), err error) {
		if _, ok := names[s]; !ok {
			err = ErrProhibitedName
			return
		}
		wr = &StringBuffer{
			strings.NewReader(s),
			&strings.Builder{},
		}
		cancel = func() { names[s]++ }
		return
	}
}

func TestConnectionManager_Open(t *testing.T) {
	words := map[string]int{
		"serial-0-0":   0,
		"serial-0-0-0": 0,
		"serial-1-0-1": 0,
	}
	closedCtx, cancel := context.WithCancel(context.Background())
	cancel()
	type fields struct {
		Context  context.Context
		provider ConnectionProvider
	}
	type args struct {
		name string
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantErr      bool
		managerClose bool
	}{
		{
			name: "Mock provider with string io",
			fields: fields{
				Context:  context.Background(),
				provider: GetMockProvider(words),
			},
			args:    args{name: "serial-0-0"},
			wantErr: false,
		},
		{
			name: "Mock provider context timed out",
			fields: fields{
				Context:  closedCtx,
				provider: GetMockProvider(words),
			},
			args:    args{name: "serial-0-0-0"},
			wantErr: false,
		},
		{
			name: "Mock provider with mismatching name",
			fields: fields{
				Context:  context.Background(),
				provider: GetMockProvider(words),
			},
			args:    args{name: "mismatch"},
			wantErr: true,
		},
		{
			name: "Closing from manager side",
			fields: fields{
				Context:  context.Background(),
				provider: GetMockProvider(words),
			},
			args:         args{name: "serial-1-0-1"},
			managerClose: true,
		},
		{
			name: "Closing from manager side with mismatching name",
			fields: fields{
				Context:  context.Background(),
				provider: GetMockProvider(words),
			},
			args:         args{name: "mismatch"},
			wantErr:      true,
			managerClose: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewManager(tt.fields.Context, tt.fields.provider)
			cm.Open(tt.args.name)
			// pick from cache once again
			got, err := cm.Open(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectionManager.Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// must be marked as opened or closed, accordingly
			if tt.wantErr == cm.IsOpen(tt.args.name) {
				t.Errorf("ConnectionManager.IsOpen() open/closed unexpectedly")
				return
			}
			// producer should be nil if something went wrong
			if got == nil && !tt.wantErr {
				t.Errorf("ConnectionManager.Open() got nil Provider but wantErr is false")
				return
			}
			// // try to close from manager's side
			if tt.managerClose {
				if err := cm.Close(tt.args.name); err != nil && !tt.wantErr {
					t.Errorf("ConnectionManager.Close() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}
			// otherwise, try to close from the producer's side
			if !tt.managerClose && got != nil {
				if err := got.Close(); err != nil {
					t.Errorf("DuplexProducer.Close() error = %v", err)
					return
				}
			}
			if got == nil {
				return
			}
			if words[tt.args.name] != 1 {
				t.Error("Cancel should have been called once: got=", words[tt.args.name])
			}
			// interface methods checks
			for range got.Data() {
			}
			for err := range got.Err() {
				// among these examples, no error should
				// have been raised
				t.Error(err)
				return
			}
			if err := got.Close(); err == nil {
				t.Error("DuplexProducer.Close() should return error on second call")
				return
			}
			if id := got.ID(); id != tt.args.name {
				t.Errorf("DuplexProducer.ID() = %v, want %v", id, tt.args.name)
				return
			}
		})
	}
}

func TestConnectionManager_Writing(t *testing.T) {
	words := map[string]int{
		"short-msg":                     0,
		"serial-port-to-buffer-mapping": 0,
	}
	type fields struct {
		Context  context.Context
		provider ConnectionProvider
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Mock provider with string io",
			fields: fields{
				Context:  context.Background(),
				provider: GetMockProvider(words),
			},
			args: args{name: "short-msg"},
		},
		{
			name: "Closing from manager side",
			fields: fields{
				Context:  context.Background(),
				provider: GetMockProvider(words),
			},
			args: args{name: "serial-port-to-buffer-mapping"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewManager(tt.fields.Context, tt.fields.provider)
			cm.Open(tt.args.name)
			// pick from cache once again
			got, err := cm.Open(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectionManager.Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// must be marked as opened or closed, accordingly
			if tt.wantErr == cm.IsOpen(tt.args.name) {
				t.Errorf("ConnectionManager.IsOpen() open/closed unexpectedly")
				return
			}
			// producer should be nil if something went wrong
			if got == nil && !tt.wantErr {
				t.Errorf("ConnectionManager.Open() got nil Provider but wantErr is false")
				return
			}
			tokens := strings.Split(tt.args.name, "-")
			log.Println(tokens)
			for _, token := range tokens {
				got.Writer() <- []byte(token)
			}
			// this one is extremely important:
			// when testing with string buffers, one must
			// make sure that message was successfully
			// consumed by the writer. Here, we want to check that
			// everything was indeed written to the buffer
			// but asking the builder asynchronously might be
			// dangerous as handler might be still processing the writer!
			<-got.Err()
			// got serial10, want serial101
			if buffer, ok := got.(*connHandler).SerialConnection.ReadWriter.(*StringBuffer); ok {
				expectedInBuf := strings.Join(tokens, "")
				if b := buffer.Builder.String(); b != expectedInBuf {
					t.Errorf("StringBuffer.Builder.String() = %v, want %v", b, expectedInBuf)
					return
				}
			}
		})
	}
}
