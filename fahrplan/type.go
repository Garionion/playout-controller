package fahrplan

import (
	"time"
)

type PlayoutJob struct {
	ID       int `json:"id"`
	Start    time.Time `json:"start"`
	Duration time.Duration `json:"duration"`
	Source   string `json:"source"`
	Version  string `json:"version"`
	Room     string `json:"room"`
}

type Fahrplan struct {
	Schedule Schedule `json:"schedule"`
}

type Persons struct {
	ID         int    `json:"id"`
	PublicName string `json:"public_name"`
}
type Talk struct {
	URL              string        `json:"url"`
	ID               int          `json:"id"`
	GUID             string        `json:"guid"`
	Logo             string		   `json:"logo"`
	Date             time.Time     `json:"date"`
	Start            string        `json:"start"`
	Duration         string        `json:"duration"`
	Room             string        `json:"room"`
	Slug             string        `json:"slug"`
	Title            string        `json:"title"`
	Subtitle         string        `json:"subtitle"`
	Track            string        `json:"track"`
	Type             string        `json:"type"`
	Language         string        `json:"language"`
	Abstract         string        `json:"abstract"`
	Description      string        `json:"description"`
	RecordingLicense string        `json:"recording_license"`
	DoNotRecord      bool          `json:"do_not_record"`
	Persons          []Persons     `json:"persons"`
	Links            []interface{} `json:"links"`
	Attachments      []interface{} `json:"attachments"`
}

type Room []Talk

type Links struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
type Attachments struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
type Days struct {
	Index    int       			`json:"index"`
	Date     string    			`json:"date"`
	DayStart time.Time 			`json:"day_start"`
	DayEnd   time.Time 			`json:"day_end"`
	Rooms    map[string]Room	`json:"rooms"`
}
type Conference struct {
	Acronym          string `json:"acronym"`
	Title            string `json:"title"`
	Start            string `json:"start"`
	End              string `json:"end"`
	DaysCount        int    `json:"daysCount"`
	TimeslotDuration string `json:"timeslot_duration"`
	Days             []Days `json:"days"`
}
type Schedule struct {
	Version    string     `json:"version"`
	BaseURL    string     `json:"base_url"`
	Conference Conference `json:"conference"`
}
