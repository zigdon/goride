package goride

import (
	// "bytes"
	// "encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"gopkg.in/ini.v1"
)

type Client struct{}

type RWGPS struct {
	token    string
	authUser *User
	config   *Config
	client   *Client
}

type Config struct {
	email    string
	password string
	keyName  string
	authPath string
	cfgPath  string
}

type User struct {
	Id   int
	Name string
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

func New(cfgPath string) (*RWGPS, error) {
	iniData, err := ini.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("error loading ini file from %q: %v", cfgPath, err)
	}
	cfg := &Config{
		cfgPath: cfgPath,
	}

	for _, name := range iniData.SectionStrings() {
		switch name {
		case "Auth":
			cfg.email = iniData.Section("Auth").Key("email").String()
			cfg.password = iniData.Section("Auth").Key("password").String()
		case "Token":
			cfg.keyName = iniData.Section("Token").Key("name").String()
			cfg.authPath = iniData.Section("Token").Key("path").String()
		}
	}
	r := &RWGPS{config: cfg, client: &Client{}}

	return r, r.Auth()
}

func (r *RWGPS) Auth() error {
	return nil
}

func (r *RWGPS) GetMyRides() ([]Ride, error) {
	return r.GetRides(r.authUser.Id)
}

func (r *RWGPS) GetRides(id int) ([]Ride, error) {
	return nil, nil
}

func (c *Client) Get(base string, args url.Values) ([]byte, error) {
	uri := base
	if len(args) > 0 {
		uri += "?" + args.Encode()
	}
	resp, err := http.Get(uri)
	if err != nil {
		return nil, fmt.Errorf("error in GET %q: %v", base, err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return body, nil
}
