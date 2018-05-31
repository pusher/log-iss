package main

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/bmizerany/lpx"
)

const (
	// LogplexDefaultHost is the default host from logplex:
	// https://github.com/heroku/logplex/blob/master/src/logplex_http_drain.erl#L443
	logplexDefaultHost = "host"
)

var nilVal = []byte(`- `)
var queryParams = []string{"index", "source", "sourcetype"}

// Fix function to convert post data to length prefixed syslog frames
func fix(req *http.Request, r io.Reader, remoteAddr string, logplexDrainToken string, metadataId string) ([]byte, error) {
	var messageWriter bytes.Buffer
	var messageLenWriter bytes.Buffer

	lp := lpx.NewReader(bufio.NewReader(r))
	for lp.Next() {
		header := lp.Header()

		// LEN SP PRI VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID SP STRUCTURED-DATA MSG
		messageWriter.Write(header.PrivalVersion)
		messageWriter.WriteString(" ")
		messageWriter.Write(header.Time)
		messageWriter.WriteString(" ")
		if string(header.Hostname) == logplexDefaultHost && logplexDrainToken != "" {
			messageWriter.WriteString(logplexDrainToken)
		} else {
			messageWriter.Write(header.Hostname)
		}
		messageWriter.WriteString(" ")
		messageWriter.Write(header.Name)
		messageWriter.WriteString(" ")
		messageWriter.Write(header.Procid)
		messageWriter.WriteString(" ")
		messageWriter.Write(header.Msgid)
		messageWriter.WriteString(" [origin ip=\"")
		messageWriter.WriteString(remoteAddr)
		messageWriter.WriteString("\"]")

		// Add metadata from query parameters
		if metadataId != "" {
			foundMetadata := false
			for _, k := range queryParams {
				v := req.FormValue(k)
				if v != "" {
					if !foundMetadata {
						messageWriter.WriteString("[")
						messageWriter.WriteString(metadataId)
						foundMetadata = true
					}
					messageWriter.WriteString(" ")
					messageWriter.WriteString(k)
					messageWriter.WriteString("=\"")
					messageWriter.WriteString(v)
					messageWriter.WriteString("\"")
				}
			}
			if foundMetadata {
				messageWriter.WriteString("]")
			}
		}

		b := lp.Bytes()
		if len(b) >= 2 && bytes.Equal(b[0:2], nilVal) {
			messageWriter.Write(b[1:])
		} else if len(b) > 0 {
			if b[0] != '[' {
				messageWriter.WriteString(" ")
			}
			messageWriter.Write(b)
		}
		if len(b) == 0 || b[len(b)-1] != '\n' {
			messageWriter.WriteString("\n")
		}

		messageLenWriter.WriteString(strconv.Itoa(messageWriter.Len()))
		messageLenWriter.WriteString(" ")
		messageWriter.WriteTo(&messageLenWriter)
	}

	return messageLenWriter.Bytes(), lp.Err()
}
