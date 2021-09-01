package goride

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type rwgpsHandler struct {
	t       *testing.T
	mu      *sync.Mutex
	static  map[string]string
	dynamic map[string]func(string, url.Values) string
}

func (h rwgpsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	path := r.URL.Path
	res, hasStatic := h.static[path]
	f, hasDynamic := h.dynamic[path]

	if hasStatic {
		fmt.Fprintf(w, res)
	} else if hasDynamic {
		fmt.Fprintf(w, f(r.URL.Path, r.URL.Query()))
	} else {
		w.Header().Add("status", "404 not found")
		fmt.Fprintf(w, "404 Not found: %q", path)
	}
}

func defaultAuth(p string, v url.Values) string {
	u := getTestData("current.json")
	if v.Get("auth_token") == "beef1337" {
		return u
	} else if v.Get("email") == "test@example.com" && v.Get("password") == "supers3cret" {
		return u
	} else {
		return "401 bad auth"
	}
}

func startServer(t *testing.T, static map[string]string, dynamic map[string]func(string, url.Values) string) *httptest.Server {
	if _, ok := dynamic["/users/current.json"]; !ok {
		if dynamic == nil {
			dynamic = make(map[string]func(string, url.Values) string)
		}
		dynamic["/users/current.json"] = defaultAuth
	}
	handler := rwgpsHandler{
		static:  static,
		dynamic: dynamic,
		mu:      &sync.Mutex{},
		t:       t,
	}
	return httptest.NewServer(handler)
}

func testConfig(path string) *Config {
	return &Config{
		CfgPath:  path,
		Email:    "test@example.com",
		Password: "supers3cret",
		KeyName:  "test key",
	}
}

func testObj(server string) *RWGPS {
	return &RWGPS{config: testConfig(""), client: &Client{server: server}}
}

func getTestData(name string) string {
	data, err := ioutil.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		log.Fatalf("Error reading testdata/%s: %v", name, err)
	}

	return string(data)
}

func TestGet(t *testing.T) {
	server := startServer(t,
		map[string]string{
			"/":     "test",
			"/path": "something",
		},
		nil)
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
		"name = \"test key\"",
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
	server := startServer(t, nil, nil)
	defer server.Close()

	tests := []struct {
		desc     string
		password string
		token    string
		wantErr  bool
	}{
		{
			desc:     "Good",
			password: "supers3cret",
		},
		{
			desc:  "Bad password, with good token",
			token: "beef1337",
		},
		{
			desc:     "Bad password, no token",
			password: "12345",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			r := testObj(server.URL)
			r.config.Password = tc.password
			if tc.token != "" {
				r.authUser = &User{AuthToken: tc.token}
			}
			r.Auth()
			if tc.wantErr {
				if r.authUser != nil && r.authUser.AuthToken != "" {
					t.Fatal("Logged in badly")
				}
				return
			}
			if r.authUser == nil || r.authUser.AuthToken == "" {
				t.Fatal("Failed to log in")
			}
			if r.authUser.ID == 0 {
				t.Errorf("Bad user id %d", r.authUser.ID)
			}
			if r.authUser.TotalTrips == 0 {
				t.Errorf("Bad total trips %d", r.authUser.TotalTrips)
			}
		})
	}
}

func TestGetRide(t *testing.T) {
	ride := getTestData("trip.json")
	server := startServer(t,
		map[string]string{"/trips/94.json": ride},
		nil)
	defer server.Close()

	r := testObj(server.URL)
	_, err := r.GetRide(1)
	if err == nil {
		t.Errorf("didn't get an error when fetching bad ride")
	}

	got, err := r.GetRide(94)
	if err != nil {
		t.Errorf("unexpected error when fetching ride: %v", err)
	}

	if got == nil {
		t.Error("missing expected ride")
	}
}

func validRideSlim(r *RideSlim) error {
	msg := []string{}
	err := func(m string) { msg = append(msg, m) }
	i := func(n int, m string) {
		if n == 0 {
			err("bad " + m)
		}
	}
	f := func(n float32, m string) {
		if n == 0 {
			err("bad " + m)
		}
	}

	i(r.ID, "ID")
	f(r.Distance, "distance")
	i(r.Duration, "duration")
	f(r.ElevationGain, "elevation gain")
	f(r.ElevationLoss, "elevation loss")
	i(r.MovingTime, "moving time")
	f(r.AvgSpeed, "average speed")
	f(r.MaxSpeed, "max speed")

	if r.DepartedAt.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		err(fmt.Sprintf("unlikely started at %s", r.DepartedAt))
	}

	if len(msg) > 0 {
		return fmt.Errorf("Invalid ride %d:\n"+strings.Join(msg, "\n"), r.ID)
	}

	return nil
}

func TestGetRides(t *testing.T) {
	tests := []struct {
		desc    string
		offset  int
		limit   int
		wantIDs []int
	}{
		{
			desc:    "0, 2",
			offset:  0,
			limit:   2,
			wantIDs: []int{38045212, 37648524},
		},
		{
			desc:    "1, 3",
			offset:  1,
			limit:   3,
			wantIDs: []int{37648524, 37120067, 27521845},
		},
	}

	f := func(_ string, args url.Values) string {
		offset := args.Get("offset")
		limit := args.Get("limit")
		return getTestData(fmt.Sprintf("trips%s-%s.json", offset, limit))
	}

	server := startServer(t,
		nil,
		map[string]func(string, url.Values) string{
			"/users/1/trips.json": f,
		})
	defer server.Close()
	r := testObj(server.URL)

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, count, err := r.GetRides(1, tc.offset, tc.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if count != 1273 {
				t.Errorf("wrong count: %d", count)
			}

			var gotIDs []int
			for _, ride := range got {
				if err := validRideSlim(ride); err != nil {
					t.Errorf("Bad ride data: %v", err)
				}
				gotIDs = append(gotIDs, ride.ID)
			}

			if diff := cmp.Diff(gotIDs, tc.wantIDs); diff != "" {
				t.Errorf("bad ride IDs: -want +got\n%s", diff)
			}
		})
	}
}

func TestGetCurrentUser(t *testing.T) {
	server := startServer(t, nil, nil)
	defer server.Close()
	r := testObj(server.URL)

	u, err := r.GetCurrentUser()
	if err != nil {
		t.Fatalf("couldn't get user: %v", err)
	}

	want := &User{
		ID:         1268590,
		Name:       "zigdon",
		TotalTrips: 3073,
		AuthToken:  "ffffff",
		Gear: []Gear{
			{239758, "Surly"},
			{255732, "TCR"},
			{256907, "Folder"},
			{256908, "Surly w/Trailer"},
		},
	}

	if diff := cmp.Diff(want, u); diff != "" {
		t.Errorf("bad user: -want +got\n%s", diff)
	}

}
