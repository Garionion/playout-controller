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

func GetUpcoming(jobs map[int]PlayoutJob, when time.Duration) map[int]PlayoutJob{
	upcoming := map[int]PlayoutJob{}
	for _, job := range jobs{
		if job.Start.After(time.Now()) && job.Start.Before(time.Now().Add(when)) {
			upcoming[job.ID] = job
		}
	}
	return upcoming
}

func GetSchedule(schedule *Fahrplan, url string) error{
	resp, err := http.Get(url) //nolint:gosec
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return fmt.Errorf("Got not OK: %s", resp.Status)
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

func ConvertScheduleToPLayoutJobs(schedule *Fahrplan) map[int]PlayoutJob {
	jobs := map[int]PlayoutJob{}
	version := schedule.Schedule.Version

	for _, day := range schedule.Schedule.Conference.Days {
		for roomName, r := range day.Rooms {
			for _, talk := range r {
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
				minute, err := time.ParseDuration(d[1] + "h")
				if err != nil {
					log.Printf("Cannot parse minute part of duration %s for %v: %s", talk.Duration, talk.ID, err)
					continue
				}
				duration := hour + minute
				job := PlayoutJob{
					ID:       talk.ID,
					Start:    talk.Date,
					Duration: duration,
					Source:   "",
					Version:  version,
					Room:     roomName,
				}
				jobs[talk.ID] = job
			}
		}
	}
	return jobs
}