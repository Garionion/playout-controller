package fahrplan

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
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

func setNextRoomTalkStart(jobs map[int]PlayoutJob) map[int]PlayoutJob {
	intermediate := make(map[string]map[time.Time]int)
	roomStartArray := make(map[string][]time.Time)
	sorted := make(map[string][]int)
	for id, job := range jobs {
		if intermediate[job.Room] == nil {
			intermediate[job.Room] = make(map[time.Time]int)
		}
		if roomStartArray[job.Room] == nil {
			roomStartArray[job.Room] = []time.Time{}
		}
		intermediate[job.Room][job.Start] = id
		roomStartArray[job.Room] = append(roomStartArray[job.Room], job.Start)
	}
	for roomName, room := range roomStartArray {
		sort.Slice(room, func(i, j int) bool {
			return room[i].Before(room[j])
		})
		for _, t := range room {
			sorted[roomName] = append(sorted[roomName], intermediate[roomName][t])
		}
		for index, jobID := range sorted[roomName] {
			if index == len(sorted[roomName])-1 {
				continue
			}
			job := jobs[jobID]
			nextTalk := sorted[roomName][index+1]
			job.Next = jobs[nextTalk].Start
			jobs[jobID] = job
		}
	}
	return jobs
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
	return setNextRoomTalkStart(jobs)
}
