package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/TroutSoftware/rx/rxtest"
)

const usage = `
Concatenates HAR files into a single entry, suitable for replay during testing.

Usage:
  harcat [-f FILTER] [-datum FILE] HARFILE...

Options:
  HARFILE               HAR file as exported by the browser 
  -d -datum FILE        Include an initial datum script entry
  -f FILTER             Apply filter on document

Filtering:
 Requests and replies are filtered by strict equality on their headers.
 The format expects a key=value entry, where the value is a regexp.

 Multiple filters can used by repeating the -f argument.

Example Usage:
 harcat -f User-Agent="Mozilla/5.0.*" originfile
`

func main() {
	var (
		datum   string
		filters []func([]HarHeader) bool
	)

	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.StringVar(&datum, "datum", "", "datum")
	flag.StringVar(&datum, "d", "", "datum")
	flag.Func("f", "f", func(s string) error {
		name, match, ok := strings.Cut(s, "=")
		if !ok {
			return fmt.Errorf("invalid filter %s: expect key=value", s)
		}
		re, err := regexp.Compile(match)
		if err != nil {
			return fmt.Errorf("invalid filter %s: %w", s, err)
		}

		filters = append(filters, func(headers []HarHeader) bool {
			s := slices.IndexFunc(headers, func(h HarHeader) bool { return h.Name == name })
			if s == -1 {
				return false
			}
			return re.MatchString(headers[s].Value)
		})
		return nil
	})

	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()

		exiterr("no file to concatenate")
	}

	var ds io.ReadCloser
	if datum != "" {
		fh, err := os.Open(datum)
		if err != nil {
			exiterr("reading datum %s: %s", datum, err)
		}
		ds = fh
	}
	t := rxtest.StartTrace(ds)

	for _, f := range flag.Args() {
		dt, err := os.ReadFile(f)
		if err != nil {
			exiterr("reading %s: %s", f, err)
		}
		var doc HarDoc
		if err := json.Unmarshal(dt, &doc); err != nil {
			exiterr("reading %s: %s", f, err)
		}
		if len(filters) > 0 {
		loopEntry:
			for _, e := range doc.Log.Entries {
				for _, f := range filters {
					if f(e.Request.Headers) || f(e.Response.Headers) {
						req, resp, err := httpify(e)
						if err != nil {
							exiterr("invalid HAR entry")
						}
						t.WriteRecord(req, resp)
						continue loopEntry
					}
				}
			}
		} else {
			for _, ets := range doc.Log.Entries {
				req, resp, err := httpify(ets)
				if err != nil {
					exiterr("invalid query")
				}
				t.WriteRecord(req, resp)
			}
		}
	}
}

func exiterr(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

// read a standard library http request/response pair from a HAR entry
func httpify(et HarEntry) (*http.Request, *http.Response, error) {
	req, err := http.NewRequest(et.Request.Method, et.Request.URL, strings.NewReader(et.Request.PostData.Text))
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}
	for _, h := range et.Request.Headers {
		req.Header.Add(h.Name, h.Value)
	}
	req.Header = rxtest.ScrubRequestHeaders(req.Header)

	resp := http.Response{
		Request:    req,
		StatusCode: et.Response.Status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(et.Response.Content.Text)),
	}
	for _, h := range et.Response.Headers {
		resp.Header.Add(h.Name, h.Value)
	}
	resp.Header = rxtest.ScrubResponseHeaders(resp.Header)

	return req, &resp, nil
}

type HarHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HarEntry struct {
	Request struct {
		Method   string      `json:"method"`
		URL      string      `json:"url"`
		Headers  []HarHeader `json:"headers"`
		PostData struct {
			Text string `json:"text"`
		} `json:"postData"`
		QueryString any `json:"queryString"`
	} `json:"request"`

	Response struct {
		Status  int         `json:"status"`
		Headers []HarHeader `json:"headers"`
		Content struct {
			Text        string `json:"text"`
			RedirectURL string `json:"redirectURL"`
		} `json:"content"`
	} `json:"response"`
}

// HarDoc represents the part of the HAR file we care about
type HarDoc struct {
	Log struct {
		Entries []HarEntry `json:"entries"`
	} `json:"log"`
}
