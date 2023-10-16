package models

import (
	"time"
)

type WeatherAPIResponse struct {
	WeatherResponse  WeatherResponse  `json:"WeatherResponse"`
	ForecastResponse ForecastResponse `json:"ForecastResponse"`
	CellWeight       int              `json:"CellWeight"`
}

type WeatherResponse struct {
	Location Location `json:"location"`
	Current  Current  `json:"current"`
}

type Location struct {
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Country        string  `json:"country"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	TzID           string  `json:"tz_id"`
	LocaltimeEpoch int64   `json:"localtime_epoch"`
	Localtime      string  `json:"localtime"`
}

type Current struct {
	LastUpdatedEpoch int     `json:"last_updated_epoch"`
	LastUpdated      string  `json:"last_updated"`
	TempC            float64 `json:"temp_c"`
	TempF            float64 `json:"temp_f"`
	IsDay            int     `json:"is_day"`
	Condition        struct {
		Text string `json:"text"`
		Icon string `json:"icon"`
		Code int    `json:"code"`
	} `json:"condition"`
	WindMph    float64 `json:"wind_mph"`
	WindKph    float64 `json:"wind_kph"`
	WindDegree int     `json:"wind_degree"`
	WindDir    string  `json:"wind_dir"`
	PressureMb float64 `json:"pressure_mb"`
	PressureIn float64 `json:"pressure_in"`
	PrecipMm   float64 `json:"precip_mm"`
	PrecipIn   float64 `json:"precip_in"`
	Humidity   int     `json:"humidity"`
	Cloud      int     `json:"cloud"`
	FeelslikeC float64 `json:"feelslike_c"`
	FeelslikeF float64 `json:"feelslike_f"`
	VisKm      float64 `json:"vis_km"`
	VisMiles   int     `json:"vis_miles"`
	Uv         int     `json:"uv"`
	GustMph    float64 `json:"gust_mph"`
	GustKph    float64 `json:"gust_kph"`
}

type ForecastResponse struct {
	Alerts Alerts `json:"alerts,omitempty"`
}

type Alerts struct {
	Alert []interface{} `json:"alert,omitempty"`
}

type CrimeMessage struct {
	CrimeID   int     `json:"crime_id"`
	CrimeName string  `json:"crime_name"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Firearm   string  `json:"firearm_involved"`
}

type GeoCell struct {
	Id                 string    `json:"id"`
	Coordinates        []float64 `json:"coordinates"`
	Latitude           float64   `json:"latitude"`
	Longitude          float64   `json:"longitude"`
	CrimeName          string    `json:"crime_name"`
	CrimeScore         int       `json:"crime_score"`
	CrimeFirearm       string    `json:"crime_firearm"`
	WeatherDescription string    `json:"weather_description"`
	WeatherCondition   int       `json:"weather_condition"`
}

type LogMessage struct {
	Timestamp time.Time
	Message   string
}

type DirectionRequest struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

type DirectionResponse struct {
	GeocodedWaypoints []GeocodedWaypoints `json:"geocoded_waypoints,omitempty"`
	Routes            []Routes            `json:"routes,omitempty"`
	Status            string              `json:"status,omitempty"`
}

type GeocodedWaypoints struct {
	GeocoderStatus string   `json:"geocoder_status,omitempty"`
	PlaceID        string   `json:"place_id,omitempty"`
	Types          []string `json:"types,omitempty"`
}

type Northeast struct {
	Lat float64 `json:"lat,omitempty"`
	Lng float64 `json:"lng,omitempty"`
}

type Southwest struct {
	Lat float64 `json:"lat,omitempty"`
	Lng float64 `json:"lng,omitempty"`
}

type Bounds struct {
	Northeast Northeast `json:"northeast,omitempty"`
	Southwest Southwest `json:"southwest,omitempty"`
}

type ArrivalTime struct {
	Text     string `json:"text,omitempty"`
	TimeZone string `json:"time_zone,omitempty"`
	Value    int    `json:"value,omitempty"`
}

type DepartureTime struct {
	Text     string `json:"text,omitempty"`
	TimeZone string `json:"time_zone,omitempty"`
	Value    int    `json:"value,omitempty"`
}

type Distance struct {
	Text  string `json:"text,omitempty"`
	Value int    `json:"value,omitempty"`
}

type Duration struct {
	Text  string `json:"text,omitempty"`
	Value int    `json:"value,omitempty"`
}

type EndLocation struct {
	Lat float64 `json:"lat,omitempty"`
	Lng float64 `json:"lng,omitempty"`
}

type StartLocation struct {
	Lat float64 `json:"lat,omitempty"`
	Lng float64 `json:"lng,omitempty"`
}

type Polyline struct {
	Points string `json:"points,omitempty"`
}

type Steps struct {
	Distance         Distance      `json:"distance,omitempty"`
	Duration         Duration      `json:"duration,omitempty"`
	EndLocation      EndLocation   `json:"end_location,omitempty"`
	HTMLInstructions string        `json:"html_instructions,omitempty"`
	Polyline         Polyline      `json:"polyline,omitempty"`
	StartLocation    StartLocation `json:"start_location,omitempty"`
	Steps            []Steps       `json:"steps,omitempty"`
	TravelMode       string        `json:"travel_mode,omitempty"`
	Maneuver         string        `json:"maneuver,omitempty"`
}

type LocationGoogle struct {
	Lat float64 `json:"lat,omitempty"`
	Lng float64 `json:"lng,omitempty"`
}

type ArrivalStop struct {
	Location LocationGoogle `json:"location,omitempty"`
	Name     string         `json:"name,omitempty"`
}

type DepartureStop struct {
	Location LocationGoogle `json:"location,omitempty"`
	Name     string         `json:"name,omitempty"`
}

type Agencies struct {
	Name  string `json:"name,omitempty"`
	Phone string `json:"phone,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Vehicle struct {
	Icon string `json:"icon,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

type Line struct {
	Agencies  []Agencies `json:"agencies,omitempty"`
	Color     string     `json:"color,omitempty"`
	Name      string     `json:"name,omitempty"`
	ShortName string     `json:"short_name,omitempty"`
	TextColor string     `json:"text_color,omitempty"`
	URL       string     `json:"url,omitempty"`
	Vehicle   Vehicle    `json:"vehicle,omitempty"`
}

type TransitDetails struct {
	ArrivalStop   ArrivalStop   `json:"arrival_stop,omitempty"`
	ArrivalTime   ArrivalTime   `json:"arrival_time,omitempty"`
	DepartureStop DepartureStop `json:"departure_stop,omitempty"`
	DepartureTime DepartureTime `json:"departure_time,omitempty"`
	Headsign      string        `json:"headsign,omitempty"`
	Line          Line          `json:"line,omitempty"`
	NumStops      int           `json:"num_stops,omitempty"`
}

type Legs struct {
	ArrivalTime       ArrivalTime   `json:"arrival_time,omitempty"`
	DepartureTime     DepartureTime `json:"departure_time,omitempty"`
	Distance          Distance      `json:"distance,omitempty"`
	Duration          Duration      `json:"duration,omitempty"`
	EndAddress        string        `json:"end_address,omitempty"`
	EndLocation       EndLocation   `json:"end_location,omitempty"`
	StartAddress      string        `json:"start_address,omitempty"`
	StartLocation     StartLocation `json:"start_location,omitempty"`
	Steps             []Steps       `json:"steps,omitempty"`
	TrafficSpeedEntry []any         `json:"traffic_speed_entry,omitempty"`
	ViaWaypoint       []any         `json:"via_waypoint,omitempty"`
}

type OverviewPolyline struct {
	Points string `json:"points,omitempty"`
}

type Routes struct {
	Bounds           Bounds           `json:"bounds,omitempty"`
	Copyrights       string           `json:"copyrights,omitempty"`
	Legs             []Legs           `json:"legs,omitempty"`
	OverviewPolyline OverviewPolyline `json:"overview_polyline,omitempty"`
	Summary          string           `json:"summary,omitempty"`
	Warnings         []string         `json:"warnings,omitempty"`
	WaypointOrder    []any            `json:"waypoint_order,omitempty"`
}
