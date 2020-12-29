package main

import (
	"bytes"
	"github.com/grafov/bcast"
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
	Source  string    `json:"source"`
	ID      int       `json:"id"`
	Version string    `json:"version"`
}

//nolint:funlen
func schedule(jobs map[int]fahrplan.PlayoutJob, cfg *Configuration, scheduledJobs map[int]fahrplan.ScheduledJob) map[int]fahrplan.ScheduledJob {
	servers := cfg.PlayoutServers
	defaultRoom, defRoomExist := servers[""]
	for _, job := range jobs {
		roomURL, ok := servers[job.Room]
		if !ok {
			if defRoomExist {
				log.Printf("server for Room %s not found, using default Room\n", job.Room)
				roomURL = defaultRoom
			} else {
				log.Printf("server for Room %s not found\n", job.Room)
				continue
			}
		}
		var postPadding time.Duration
		if !job.Next.IsZero() && cfg.MaxPostPadding > job.Next.Sub(job.Start.Add(job.Duration)) {
			postPadding = job.Next.Sub(job.Start.Add(job.Duration))
		} else {
			postPadding = cfg.MaxPostPadding
		}
		reqBody, err := json.Marshal(Job{
			StartAt: job.Start.Add(-cfg.PrePadding),
			StopAt:  job.Start.Add(job.Duration).Add(postPadding),
			Source:  job.Source,
			ID:      job.ID,
			Version: job.Version,
		})
		if err != nil {
			log.Printf("marshalling of job %v failed: %v\n", job.ID, err)
			continue
		}

		u, _ := url.Parse(roomURL)
		u.Path = path.Join(u.Path, "schedulePlayout")
		c := http.Client{Timeout: 5 * time.Second}
		resp, err := c.Post(u.String(), "application/json", bytes.NewBuffer(reqBody)) //nolint:gosec
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

		var scheduledJob fahrplan.ScheduledJob
		jsonErr := json.Unmarshal(body, &scheduledJob)
		if jsonErr != nil {
			log.Printf("Can't Unmarshall JSON for %v: %v", job.ID, jsonErr)
			continue
		}
		log.Printf("Scheduled %v", job.ID)
		scheduledJob.Room = job.Room
		scheduledJobs[job.ID] = scheduledJob
	}
	return scheduledJobs
}

func removeAlreadyScheduledJobs(jobs map[int]fahrplan.PlayoutJob, scheduled map[int]fahrplan.ScheduledJob) map[int]fahrplan.PlayoutJob {
	for id := range scheduled {
		if jobs[id].Version != scheduled[id].Version {
			continue
		}
		delete(jobs, id)
	}
	return jobs
}

func scheduler(cfg *Configuration, upcomingChannel *bcast.Member, scheduledChannel *bcast.Member) chan struct{} {
	quit := make(chan struct{})
	go func(cfg *Configuration, upcomingChannel *bcast.Member, scheduledChannel *bcast.Member) {
		scheduled := make(map[int]fahrplan.ScheduledJob)
		for upcoming := range upcomingChannel.Read {
			u := upcoming.(map[int]fahrplan.PlayoutJob)
			if !cfg.AutoSchedule {
				continue
			}
			toSchedule := removeAlreadyScheduledJobs(u, scheduled)
			if len(toSchedule) == 0 {
				log.Println("Nothing new to Schedule")
			} else {
				scheduled = schedule(toSchedule, cfg, scheduled)
				scheduledChannel.Send(scheduled)
			}
		}
	}(cfg, upcomingChannel, scheduledChannel)
	return quit
}
