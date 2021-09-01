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
	ID   int
	Name string
}

type User struct {
	ID         int
	Name       string
	AuthToken  string `json:"auth_token"`
	Gear       []Gear
	TotalTrips int `json:"trips_included_in_totals_count"`
}

type Metrics struct {
	AscentTime    int
	DescentTime   int
	Calories      int
	Distance      float32
	Duration      time.Duration
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

type RideSlim struct {
	ID                       int       `json:"id"`
	GroupMembershipID        int       `json:"group_membership_id"`
	RouteID                  int       `json:"route_id"`
	CreatedAt                time.Time `json:"created_at"`
	GearID                   int       `json:"gear_id"`
	DepartedAt               time.Time `json:"departed_at"`
	Duration                 int       `json:"duration"`
	Distance                 float32   `json:"distance"`
	ElevationGain            float32   `json:"elevation_gain"`
	ElevationLoss            float32   `json:"elevation_loss"`
	Visibility               int       `json:"visibility"`
	Description              string    `json:"description"`
	IsGps                    bool      `json:"is_gps"`
	Name                     string    `json:"name"`
	MaxHr                    float32   `json:"max_hr"`
	MinHr                    float32   `json:"min_hr"`
	AvgHr                    float32   `json:"avg_hr"`
	MaxCad                   float32   `json:"max_cad"`
	MinCad                   float32   `json:"min_cad"`
	AvgCad                   float32   `json:"avg_cad"`
	AvgSpeed                 float32   `json:"avg_speed"`
	MaxSpeed                 float32   `json:"max_speed"`
	MovingTime               int       `json:"moving_time"`
	Processed                bool      `json:"processed"`
	AvgWatts                 float32   `json:"avg_watts"`
	MaxWatts                 float32   `json:"max_watts"`
	MinWatts                 float32   `json:"min_watts"`
	IsStationary             bool      `json:"is_stationary"`
	Calories                 int       `json:"calories"`
	UpdatedAt                time.Time `json:"updated_at"`
	TimeZone                 string    `json:"time_zone"`
	FirstLng                 float64   `json:"first_lng"`
	FirstLat                 float64   `json:"first_lat"`
	LastLng                  float64   `json:"last_lng"`
	LastLat                  float64   `json:"last_lat"`
	UserID                   int       `json:"user_id"`
	DeletedAt                time.Time `json:"deleted_at"`
	SwLng                    float32   `json:"sw_lng"`
	SwLat                    float32   `json:"sw_lat"`
	NeLng                    float32   `json:"ne_lng"`
	NeLat                    float32   `json:"ne_lat"`
	TrackID                  string    `json:"track_id"`
	PostalCode               string    `json:"postal_code"`
	Locality                 string    `json:"locality"`
	AdministrativeArea       string    `json:"administrative_area"`
	CountryCode              string    `json:"country_code"`
	SourceType               string    `json:"source_type"`
	LikesCount               int       `json:"likes_count"`
	HighlightedPhotoID       int       `json:"highlighted_photo_id"`
	HighlightedPhotoChecksum string    `json:"highlighted_photo_checksum"`
	UtcOffset                int       `json:"utc_offset"`
}

type Ride struct {
	ID          int
	Started     time.Time `json:"departed_at"`
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
	r := &RWGPS{config: cfg, client: &Client{server: "https://ridewithgps.com"}}

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
	log.Printf("Logged in as %q (%d)", u.Name, u.ID)
	r.authUser = u

	return nil
}

func (r *RWGPS) GetRides(user, offset, limit int) ([]*RideSlim, int, error) {
	res, err := r.Get(fmt.Sprintf("/users/%d/trips.json", user),
		url.Values{
			"offset": []string{fmt.Sprintf("%d", offset)},
			"limit":  []string{fmt.Sprintf("%d", limit)},
		})
	if err != nil {
		return nil, 0, fmt.Errorf("error getting rides %d+%d for %d: %v", offset, limit, user, err)
	}

	var resStruct struct {
		Count int         `json:"results_count"`
		Rides []*RideSlim `json:"results"`
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
