// Copyright 2014 Manu Martinez-Almeida.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package common

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// Server-Sent Events
// W3C Working Draft 29 October 2009
// http://www.w3.org/TR/2009/WD-eventsource-20091029/

var writeContentType = []string{"text/event-stream"}
var noCache = []string{"no-cache"}

var fieldReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\r", "\\r")

var dataReplacer = strings.NewReplacer(
	"\n", "\n",
	"\r", "\\r")

var (
	sseDataPrefix = []byte("data: ")
	sseDataSuffix = []byte("\n\n")
)

// CustomEvent does not synchronize writes to the response writer. Streaming
// callers must serialize event writes at the stream level.
type CustomEvent struct {
	Event string
	Id    string
	Retry uint
	Data  interface{}
}

func encode(writer io.Writer, event CustomEvent) error {
	return writeData(writer, event.Data)
}

func writeData(w io.Writer, data interface{}) error {
	s := data.(string)
	if err := writeSSEDataValue(w, s); err != nil {
		return err
	}
	if !strings.HasPrefix(s, "data") {
		return nil
	}
	_, err := w.Write(sseDataSuffix)
	return err
}

// writeSSEDataValue writes an SSE data-field value.
//
// Replacer is skipped unless the payload contains '\r'.
//
// Premise: pass-through chunks are single bufio.Scanner (ScanLines) records.
// ScanLines splits on '\n' and strips a trailing '\r' from "\r\n", so a data
// line itself contains no '\n'. JSON from encoding/json also contains no raw
// CR/LF. dataReplacer only transforms '\r' → `\\r` ('\n' is identity); the
// IndexByte check keeps the rare in-line CR case byte-identical to the old
// full-scan Replacer path.
func writeSSEDataValue(w io.Writer, s string) error {
	if strings.IndexByte(s, '\r') >= 0 {
		_, err := dataReplacer.WriteString(w, s)
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

// WriteSSEData writes one SSE data event: "data: " + payload + "\n\n".
func WriteSSEData(w http.ResponseWriter, payload string) error {
	CustomEvent{}.WriteContentType(w)
	if _, err := w.Write(sseDataPrefix); err != nil {
		return err
	}
	if err := writeSSEDataValue(w, payload); err != nil {
		return err
	}
	_, err := w.Write(sseDataSuffix)
	return err
}

// WriteSSEDataBytes is WriteSSEData for a pre-encoded payload, avoiding a
// []byte→string conversion on the hot path.
func WriteSSEDataBytes(w http.ResponseWriter, payload []byte) error {
	CustomEvent{}.WriteContentType(w)
	if _, err := w.Write(sseDataPrefix); err != nil {
		return err
	}
	if bytes.IndexByte(payload, '\r') >= 0 {
		if err := writeSSEDataValue(w, string(payload)); err != nil {
			return err
		}
	} else if _, err := w.Write(payload); err != nil {
		return err
	}
	_, err := w.Write(sseDataSuffix)
	return err
}

func (r CustomEvent) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	return encode(w, r)
}

func (r CustomEvent) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	header["Content-Type"] = writeContentType

	if _, exist := header["Cache-Control"]; !exist {
		header["Cache-Control"] = noCache
	}
}
