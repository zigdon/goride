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
	authUser *User
	config   *Config
	client   *Client
}

type Config struct {
	Email    string
	Password string
	KeyName  string
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

type Metrics struct {
	AscentTime    int
	DescentTime   int
	Calories      int
	Distance      float32
	Duration      int
	ElevationGain float32 `json:"ele_gain"`
	ElevationLoss float32 `json:"ele_loss"`
	Grade         struct {
		Avg float32
		Max float32
		Min float32
	}
	MovingTime int
	Speed      struct {
		Avg float32
		Max float32
		Min float32
	}
	Stationary bool
}

type LatLng struct {
	Lat float32
	Lng float32
}

type Ride struct {
	Id          int
	CreateAt    time.Time `json:"created_at"`
	Metrics     Metrics
	Distance    float32
	Description string
	Name        string
	BoundingBox []LatLng `json:"bounding_box"`
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
			cfg.KeyName = iniData.Section("Auth").Key("name").String()
		default:
			log.Printf("Bad section in ini: %q", name)
		}
	}

	return cfg, nil
}

func decodeJSON(data string, obj interface{}) error {
	dec := json.NewDecoder(strings.NewReader(data))

	if err := dec.Decode(obj); err != nil {
		return fmt.Errorf("error decoding json: %v\n%s", err, data)
	}

	return nil
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
	if r.authUser == nil || r.authUser.AuthToken == "" {
		log.Printf("No auth token found, logging in...")
		args := url.Values{
			"email":    []string{r.config.Email},
			"password": []string{r.config.Password},
			"apikey":   []string{r.config.KeyName},
			"version":  []string{"2"},
		}
		res, err = r.client.Get("/users/current.json", args)
	} else {
		res, err = r.Get("/users/current.json", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting current user: %v", err)
	}

	var resStruct struct{ User User }
	err = decodeJSON(res, &resStruct)

	return &resStruct.User, err
}

func (r *RWGPS) Get(method string, args url.Values) (string, error) {
	if r.authUser == nil || r.authUser.AuthToken == "" {
		err := r.Auth()
		if err != nil {
			return "", fmt.Errorf("can't auth: %v", err)
		}
	}
	if args == nil {
		args = url.Values{}
	}
	args.Add("apikey", r.config.KeyName)
	args.Add("version", "2")
	args.Add("auth_token", r.authUser.AuthToken)
	return r.client.Get(method, args)
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

func (r *RWGPS) GetRides(user, offset, limit int) ([]*Ride, int, error) {
	res, err := r.Get(fmt.Sprintf("/users/%d/trips.json", user),
		url.Values{
			"offset": []string{fmt.Sprintf("%d", offset)},
			"limit":  []string{fmt.Sprintf("%d", limit)},
		})
	if err != nil {
		return nil, 0, fmt.Errorf("error getting rides %d+%d for %d: %v", offset, limit, user, err)
	}

	var resStruct struct {
		Count int     `json:"results_count"`
		Rides []*Ride `json:"results"`
	}

	err = decodeJSON(res, &resStruct)
	return resStruct.Rides, resStruct.Count, err

}

func (r *RWGPS) GetRide(id int) (*Ride, error) {
	res, err := r.Get(fmt.Sprintf("/trips/%d.json", id), nil)
	if err != nil {
		return nil, fmt.Errorf("error getting ride id %d: %v", id, err)
	}

	var resStruct struct {
		Type string
		Trip Ride
	}

	err = decodeJSON(res, &resStruct)
	if err != nil {
		return nil, err
	}

	if resStruct.Type != "trip" {
		return nil, fmt.Errorf("unexpected result type %q", resStruct.Type)
	}

	return &resStruct.Trip, nil
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
