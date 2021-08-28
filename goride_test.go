package goride

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

type simpleHandler struct {
	t        *testing.T
	mu       *sync.Mutex
	mappings map[string]string
}

func (h simpleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	path := r.URL.Path
	res, ok := h.mappings[path]
	if !ok {
		fmt.Fprintf(w, "404 Not found")
		return
	}

	fmt.Fprintf(w, res)
}

func startServer(t *testing.T, res map[string]string) *httptest.Server {
	handler := simpleHandler{mappings: res, mu: &sync.Mutex{}, t: t}
	return httptest.NewServer(handler)
}

func TestGet(t *testing.T) {
	server := startServer(t,
		map[string]string{
			"/":     "test",
			"/path": "something",
		})
	defer server.Close()

	tests := []struct {
		desc string
		url  string
		args url.Values
		want string
	}{
		{
			desc: "root",
			url:  "/",
			want: "test",
		},
		{
			desc: "path",
			url:  "/path",
			want: "something",
		},
	}

	c := &Client{}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			res, err := c.Get(server.URL+tc.url, tc.args)
			if err != nil {
				t.Fatal(err)
			}

			if string(res) != tc.want {
				t.Errorf("Unexpected result: -want +got\n-%s\n+%s", tc.want, string(res))
			}
		})
	}

}
