package goride

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
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
		fmt.Fprintf(w, "404 Not found: %q", path)
		return
	}

	fmt.Fprintf(w, res)
}

func startServer(t *testing.T, res map[string]string) *httptest.Server {
	handler := simpleHandler{mappings: res, mu: &sync.Mutex{}, t: t}
	return httptest.NewServer(handler)
}

func testConfig(path string) *Config {
	return &Config{
		CfgPath:  path,
		Email:    "test@example.com",
		Password: "supers3cret",
		KeyName:  "test key",
		AuthPath: "/path/to/file",
	}
}

func liveObj(t *testing.T) *RWGPS {
	t.Helper()
	r, err := New("/home/zigdon/.config/ridewithgps.ini")
	if err != nil {
		t.Fatalf("can't load live config: %v", err)
	}
	r.client.server = "https://ridewithgps.com"
	t.Logf("server set to %q", r.client.server)
	return r
}

func testObj(server string) *RWGPS {
	return &RWGPS{config: testConfig(""), client: &Client{server: server}}
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

	c := &Client{server: server.URL}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			res, err := c.Get(tc.url, tc.args)
			if err != nil {
				t.Fatal(err)
			}

			if string(res) != tc.want {
				t.Errorf("Unexpected result: -want +got\n-%s\n+%s", tc.want, string(res))
			}
		})
	}
}

func TestConfig(t *testing.T) {
	cfg := strings.Join([]string{
		"[Auth]",
		"email = test@example.com",
		"password = supers3cret",
		"",
		"[Token]",
		"name = \"test key\"",
		"path = /path/to/file",
	}, "\n")

	path := filepath.Join(t.TempDir(), "cfg.ini")
	err := ioutil.WriteFile(path, []byte(cfg), 0644)
	if err != nil {
		t.Fatalf("can't write test config from %q: %v", path, err)
	}

	got, err := NewConfig(path)
	if err != nil {
		t.Fatalf("error loading config: %v", err)
	}

	want := testConfig(path)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected diff: -want +got\n%s", diff)
	}
}

func TestAuth(t *testing.T) {
	r := liveObj(t)
	r.Auth()
	if r.authUser == nil {
		t.Errorf("Failed to log in")
	}
}
