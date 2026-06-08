package rxtest

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// # HTTP record-and-replay interactions
//
// For real-world testing of front-end code, we need to be emulate the interactions
// with the actual server, without starting it (for stability).
// This is done by recording a trace, then passing it as a RoundTripper to the page.
//
// The package understands the epochal model for synchronisation:
// once a response from epoch X has been received, it is an error to send a request with an epoch < X+1.
//
// This package is inspired from OSCAR’s [httprr] system.
//
// [httprr]: https://pkg.go.dev/golang.org/x/oscar/internal/httprr

// Trace captures (and replay) server interactions
type Trace struct {
	out io.ReadWriteCloser
	err error
}

// StartTrace creates a new trace writing to stdout.
// This is mostly useful for scripts such as harcat.
func StartTrace(datum io.ReadCloser) *Trace {
	trace := &Trace{out: os.Stdout}

	fmt.Fprintf(os.Stdout, "rxtest trace v1\n")
	if datum != nil {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, datum)
		if err != nil {
			trace.err = err
		}
		datum.Close()
		fmt.Fprintf(os.Stdout, "datum %d\n", buf.Len())
		buf.WriteTo(os.Stdout)
	}
	return trace
}

func (t *Trace) WriteRecord(request *http.Request, response *http.Response) {
	var buf1, buf2 bytes.Buffer
	request.WriteProxy(&buf1)
	response.Write(&buf2)
	_, err1 := fmt.Fprintf(t.out, "rr %d %d\n", buf1.Len(), buf2.Len())
	_, err2 := buf1.WriteTo(t.out)
	_, err3 := buf2.WriteTo(t.out)
	if err := cmp.Or(err1, err2, err3); err != nil {
		t.err = err
	}
}

// ScrubRequestHeaders removes all headers not required to test the synchronisation.
func ScrubRequestHeaders(h http.Header) http.Header {
	expected := []string{"Accept", "Content-Type", "X-Epoch", "X-Transaction"}
	out := make(http.Header)
	for _, x := range expected {
		out[x] = h[x]
	}
	return out
}

// ScrubResponseHeaders removes all headers not required to test the synchronisation.
func ScrubResponseHeaders(h http.Header) http.Header {
	expected := []string{"Content-Type"}
	out := make(http.Header)
	for _, x := range expected {
		out[x] = h[x]
	}
	return out
}

// Replay the content stored in file
func Replay(file string) (io.Reader, http.RoundTripper, error) {
	bdata, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", file, err)
	}

	line, data, ok := strings.Cut(string(bdata), "\n")
	if !ok || line != "rxtest trace v1" {
		return nil, nil, fmt.Errorf("reading %s: not a trace", file)
	}
	var datum string
	replay := make(map[string]string)
	for data != "" {
		line, data, ok = strings.Cut(data, "\n")
		headers := strings.Split(line, " ")
		switch {
		default:
			return nil, nil, fmt.Errorf("reading %s: corrupt file", file)
		case len(headers) == 0:
			return nil, nil, fmt.Errorf("reading %s: corrupt file", file)
		case headers[0] == "datum" && len(headers) == 2:
			n, err := strconv.Atoi(headers[1])
			if err != nil || n > len(data) {
				return nil, nil, fmt.Errorf("reading %s: corrupt file", file)
			}
			datum, data = data[:n], data[n:]
		case headers[0] == "rr" && len(headers) == 3:
			n1, err1 := strconv.Atoi(headers[1])
			n2, err2 := strconv.Atoi(headers[2])
			if cmp.Or(err1, err2) != nil || n1 > len(data) || n2 > len(data[n1:]) {
				return nil, nil, fmt.Errorf("reading %s: corrupt file", file)
			}
			var req, resp string
			req, resp, data = data[:n1], data[n1:n1+n2], data[n1+n2:]
			replay[req] = resp
		}
	}

	return strings.NewReader(datum), nil, nil
}

type recorded struct {
	replay map[string]string
}

func (r recorded) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header = ScrubRequestHeaders(req.Header)
	var buf bytes.Buffer
	req.WriteProxy(&buf)
	srsp, ok := r.replay[buf.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected request to %s", req.RequestURI)
	}
	return http.ReadResponse(bufio.NewReader(strings.NewReader(srsp)), req)
}
