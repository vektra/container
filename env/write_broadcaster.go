package env

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"
)

type WriteBroadcaster struct {
	sync.Mutex
	buf     *bytes.Buffer
	writers map[StreamWriter]bool
}

type StreamWriter struct {
	wc     io.WriteCloser
	stream string
}

func (w *WriteBroadcaster) AddWriter(writer io.WriteCloser, stream string) {
	w.Lock()
	sw := StreamWriter{wc: writer, stream: stream}
	w.writers[sw] = true
	w.Unlock()
}

type JSONLog struct {
	Log     string    `json:"log,omitempty"`
	Stream  string    `json:"stream,omitempty"`
	Created time.Time `json:"time"`
}

func (w *WriteBroadcaster) Write(p []byte) (n int, err error) {
	w.Lock()
	defer w.Unlock()
	w.buf.Write(p)
	for sw := range w.writers {
		lp := p
		if sw.stream != "" {
			lp = nil
			for {
				line, err := w.buf.ReadString('\n')
				if err != nil {
					w.buf.Write([]byte(line))
					break
				}
				b, err := json.Marshal(&JSONLog{Log: line, Stream: sw.stream, Created: time.Now()})
				if err != nil {
					// On error, evict the writer
					delete(w.writers, sw)
					continue
				}
				lp = append(lp, b...)
			}
		}
		if n, err := sw.wc.Write(lp); err != nil || n != len(lp) {
			// On error, evict the writer
			delete(w.writers, sw)
		}
	}
	return len(p), nil
}

func (w *WriteBroadcaster) CloseWriters() error {
	w.Lock()
	defer w.Unlock()
	for sw := range w.writers {
		sw.wc.Close()
	}
	w.writers = make(map[StreamWriter]bool)
	return nil
}

func NewWriteBroadcaster() *WriteBroadcaster {
	return &WriteBroadcaster{writers: make(map[StreamWriter]bool), buf: bytes.NewBuffer(nil)}
}
