package fahrplan

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func GetUpcoming(jobs map[int]PlayoutJob, when time.Duration) map[int]PlayoutJob {
	upcoming := map[int]PlayoutJob{}
	now := time.Now()
	for _, job := range jobs {
		if job.Start.After(now) && job.Start.Before(now.Add(when)) ||
			now.After(job.Start) && now.Before(job.Start.Add(job.Duration)) {
			upcoming[job.ID] = job
		}
	}
	return upcoming
}

func GetSchedule(schedule *Fahrplan, url string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return fmt.Errorf("got not OK: %s", resp.Status)
	}

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return readErr
	}

	jsonErr := json.Unmarshal(body, schedule)
	if jsonErr != nil {
		return jsonErr
	}
	return nil
}

func ConvertScheduleToPLayoutJobs(schedule *Fahrplan, talkIDtoIngestURL map[int]string) map[int]PlayoutJob {
	jobs := map[int]PlayoutJob{}
	version := schedule.Schedule.Version

	for _, day := range schedule.Schedule.Conference.Days {
		for roomName, r := range day.Rooms {
			for _, talk := range r {
				if _, ok := talkIDtoIngestURL[talk.ID]; !ok {
					continue
				}
				d := strings.Split(talk.Duration, ":")
				// I assume that every talk has a hour and a minute part
				if len(d) != 2 {
					log.Printf("Cannot parse duration %s", talk.Duration)
					continue
				}
				hour, err := time.ParseDuration(d[0] + "h")
				if err != nil {
					log.Printf("Cannot parse hour part of duration %s for %v: %s", talk.Duration, talk.ID, err)
					continue
				}
				minute, err := time.ParseDuration(d[1] + "m")
				if err != nil {
					log.Printf("Cannot parse minute part of duration %s for %v: %s", talk.Duration, talk.ID, err)
					continue
				}
				duration := hour + minute
				job := PlayoutJob{
					ID:       talk.ID,
					Start:    talk.Date,
					Duration: duration,
					Source:   talkIDtoIngestURL[talk.ID],
					Version:  version,
					Room:     roomName,
				}
				jobs[talk.ID] = job
			}
		}
	}
	return jobs
}
