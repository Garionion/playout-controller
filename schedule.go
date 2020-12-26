package main

import (
	"bytes"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"playout-controller/fahrplan"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Job struct {
	StartAt time.Time `json:"startAt,omitempty"`
	StopAt  time.Time `json:"stopAt,omitempty"`
	Source  string `json:"source"`
	ID int `json:"id"`
	Version string `json:"version"`
}

type ScheduledJob struct {
	ID      int `json:"id"`
	Port    string `json:"port"`
	Room    string `json:"room"`
	Version string `json:"version"`
}

func schedule(jobs map[int]fahrplan.PlayoutJob, servers map[string]string, scheduledJobs map[int]ScheduledJob) map[int]ScheduledJob {
	defaultRoom, defRoomExist := servers[""]
	for _, job := range jobs {
		roomUrl, ok := servers[job.Room]
		if !ok {
			if defRoomExist {
				log.Printf("server for Room %s not found, using default Room\n", job.Room)
				roomUrl = defaultRoom
			} else {
				log.Printf("server for Room %s not found\n", job.Room)
				continue
			}
		}
		reqBody, err := json.Marshal(Job{
			StartAt: job.Start,
			StopAt:  job.Start.Add(job.Duration),
			Source:  "test",
			ID: job.ID,
			Version: job.Version,
		})
		if err != nil {
			log.Printf("marshalling of job %v failed: %v\n", job.ID, err)
			continue
		}


		u, _ := url.Parse(roomUrl)
		u.Path = path.Join(u.Path, "schedulePlayout")
		resp, err := http.Post(u.String(), "application/json", bytes.NewBuffer(reqBody)) //nolint:gosec
		if err != nil {
			log.Printf("schedule Request for %v failed: %v\n", job.ID, err)
			continue
		}
		defer resp.Body.Close()

		body, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			log.Printf("Can't read response for %v: %v\n", job.ID, readErr)
			continue
		}
		if resp.StatusCode >= 300 || resp.StatusCode < 200 {
			log.Printf("got not OK for Scheduling Job %d: %s\n%v", job.ID, resp.Status, string(body))
			continue
		}

		var scheduledJob ScheduledJob
		jsonErr := json.Unmarshal(body, &scheduledJob)
		if jsonErr != nil {
			log.Printf("Can't Unmarshall JSON for %v: %v", job.ID, jsonErr)
			continue
		}
		scheduledJob.Room = job.Room
		scheduledJobs[job.ID] = scheduledJob
	}
	return scheduledJobs
}

func removeAlreadyScheduledJobs(jobs map[int]fahrplan.PlayoutJob, scheduled map[int]ScheduledJob) map[int]fahrplan.PlayoutJob {
	for id := range scheduled {
		if jobs[id].Version != scheduled[id].Version{
			continue
		}
		delete(jobs, id)
	}
	return jobs
}

func scheduler(jobs map[int]fahrplan.PlayoutJob, scheduled map[int]ScheduledJob, cfg *Configuration, firstUpcoming chan bool) chan struct{} {
	ticker := time.NewTicker(cfg.UpcomingInterval / 2)
	quit := make(chan struct{})
	go func(jobs map[int]fahrplan.PlayoutJob, scheduled map[int]ScheduledJob, cfg *Configuration, firstUpcoming chan bool) {
		if cfg.AutoSchedule {
			<-firstUpcoming
			log.Println("Got first upcoming Jobs")
			scheduled = schedule(removeAlreadyScheduledJobs(jobs, scheduled), cfg.PlayoutServers, scheduled)
		}
		for {
			select {
			case <-ticker.C:
				if !cfg.AutoSchedule {
					continue
				}
				scheduled = schedule(removeAlreadyScheduledJobs(jobs, scheduled), cfg.PlayoutServers, scheduled)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(jobs, scheduled, cfg, firstUpcoming)
	return quit
}

