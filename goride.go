package goride

import (
	// "bytes"
	// "encoding/json"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

type Client struct {
	server string
}

type RWGPS struct {
	token    string
	authUser *User
	config   *Config
	client   *Client
}

type Config struct {
	Email    string
	Password string
	KeyName  string
	AuthPath string
	CfgPath  string
}

type Gear struct {
	Id   int
	Name string
}

type User struct {
	Id        int
	Name      string
	AuthToken string `json:"auth_token"`
	Gear      []Gear
}

type Ride struct {
	Id                                   int
	CreateAt                             time.Time
	Duration                             int
	Distance                             float32
	Description                          string
	Name                                 string
	ElevationGain, ElevationLoss         float32
	MaxSpeed, AvgSpeed                   float32
	IsStationary                         bool
	FirstLng, FirstLat, LastLng, LastLat float32
	SwLng, SwLat, NeLng, NeLat           float32
}

func NewConfig(path string) (*Config, error) {
	iniData, err := ini.LoadSources(ini.LoadOptions{UnescapeValueDoubleQuotes: true}, path)
	if err != nil {
		return nil, fmt.Errorf("error loading ini file from %q: %v", path, err)
	}
	cfg := &Config{
		CfgPath: path,
	}

	for _, name := range iniData.SectionStrings() {
		switch name {
		case "Auth":
			cfg.Email = iniData.Section("Auth").Key("email").String()
			cfg.Password = iniData.Section("Auth").Key("password").String()
		case "Token":
			cfg.KeyName = iniData.Section("Token").Key("name").String()
			cfg.AuthPath = iniData.Section("Token").Key("path").String()
		}
	}

	return cfg, nil
}

func New(cfgPath string) (*RWGPS, error) {
	cfg, err := NewConfig(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("can't load config from %q: %v", cfgPath, err)
	}
	r := &RWGPS{config: cfg, client: &Client{}}

	return r, nil
}

func (r *RWGPS) GetCurrentUser() (*User, error) {
	var res string
	var err error
	if r.token == "" {
		log.Printf("No auth token found, logging in...")
		args := url.Values{
			"email":    []string{r.config.Email},
			"password": []string{r.config.Password},
		}
		res, err = r.client.Get("/users/current.json", args)
	} else {
		res, err = r.Get("/users/current.json")
	}
	if err != nil {
		return nil, fmt.Errorf("error getting current user: %v", err)
	}

	var resStruct struct{ User User }
	dec := json.NewDecoder(strings.NewReader(res))

	if err = dec.Decode(&resStruct); err != nil {
		return nil, fmt.Errorf("error decoding json: %v\n%s", err, res)
	}

	return &resStruct.User, nil
}

func (r *RWGPS) Get(method string) (string, error) {
	if r.token == "" {
		err := r.Auth()
		if err != nil {
			return "", fmt.Errorf("can't auth: %v", err)
		}
	}
	return r.client.Get(method, url.Values{
		"apikey":     []string{r.config.KeyName},
		"version":    []string{"2"},
		"auth_token": []string{r.token},
	})
}

func (r *RWGPS) Auth() error {
	u, err := r.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("can't log in: %v", err)
	}
	log.Printf("Logged in as %q (%d)", u.Name, u.Id)
	r.authUser = u

	return nil
}

func (r *RWGPS) GetMyRides() ([]Ride, error) {
	return r.GetRides(r.authUser.Id)
}

func (r *RWGPS) GetRides(id int) ([]Ride, error) {
	return nil, nil
}

func (c *Client) Get(base string, args url.Values) (string, error) {
	var uri string
	if c.server != "" {
		uri = c.server + base
	} else {
		uri = base
	}
	if len(args) > 0 {
		uri += "?" + args.Encode()
	}
	resp, err := http.Get(uri)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			return "", fmt.Errorf("error in GET %q: %q %v", base, resp.Status, err)
		} else {
			return "", fmt.Errorf("error in GET %q: %v", base, err)
		}
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body), nil
}
