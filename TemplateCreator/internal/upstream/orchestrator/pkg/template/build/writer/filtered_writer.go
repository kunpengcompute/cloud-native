// Copyright 2023 FoundryLabs, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package writer

import (
	"bytes"
	"io"
	"strings"
)

type PrefixFilteredWriter struct {
	io.Writer

	PrefixFilter string
	buff         bytes.Buffer
}

// Write will split the input on newlines and post each line as a new log entry
// to the logger.
func (w *PrefixFilteredWriter) Write(bs []byte) (n int, err error) {
	n = len(bs)
	for len(bs) > 0 {
		bs = w.writeLine(bs)
	}

	return n, nil
}

// writeLine writes a single line from the input, returning the remaining,
// unconsumed bytes.
func (w *PrefixFilteredWriter) writeLine(line []byte) (remaining []byte) {
	idx := bytes.IndexByte(line, '\n')
	if idx < 0 {
		// If there are no newlines, buffer the entire string.
		w.buff.Write(line)

		return nil
	}

	// Split on the newline, buffer and flush the left.
	line, remaining = line[:idx], line[idx+1:]

	// Fast path: if we don't have a partial message from a previous write
	// in the buffer, skip the buffer and log directly.
	if w.buff.Len() == 0 {
		w.log(line)

		return remaining
	}

	w.buff.Write(line)

	// Log empty messages in the middle of the stream so that we don't lose
	// information when the user writes "foo\n\nbar".
	w.flush(true)

	return remaining
}

func (w *PrefixFilteredWriter) Close() error {
	return w.Sync()
}

func (w *PrefixFilteredWriter) Sync() error {
	// Don't allow empty messages on explicit Sync calls or on Close
	// because we don't want an extraneous empty message at the end of the
	// stream -- it's common for files to end with a newline.
	w.flush(false)

	return nil
}

func (w *PrefixFilteredWriter) flush(allowEmpty bool) {
	if allowEmpty || w.buff.Len() > 0 {
		w.log(w.buff.Bytes())
	}
	w.buff.Reset()
}

// log writes the buffered line to the underlying writer, filtering in only
// the prefixed messages. It removes the configured prefix from the line.
func (w *PrefixFilteredWriter) log(b []byte) {
	line := string(b)
	noPrefixLine := strings.TrimPrefix(line, w.PrefixFilter)
	if w.PrefixFilter == "" || noPrefixLine != line {
		toWrite := []byte(noPrefixLine + "\n")
		w.Writer.Write(toWrite)
	}
}
