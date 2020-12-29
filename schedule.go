package main

import (
	"context"
	"github.com/Garionion/ffmpeg-playout/api"
	"github.com/Garionion/playout-controller/fahrplan"
	"github.com/Garionion/playout-controller/store"
	"github.com/golang/protobuf/ptypes"
	"github.com/grafov/bcast"
	jsoniter "github.com/json-iterator/go"
	"log"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

//nolint:funlen
func schedule(cfg *Configuration, store *store.Store, jobs map[int]fahrplan.PlayoutJob, scheduledJobs map[int]api.ScheduledJob, addPadding bool) map[int]api.ScheduledJob {
	servers := store.GrpcClients
	defaultRoom, defRoomExist := servers[""]
	for _, job := range jobs {
		playoutClient, ok := servers[job.Room]
		if !ok {
			if defRoomExist {
				log.Printf("server for Room %s not found, using default Room\n", job.Room)
				playoutClient = defaultRoom
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
		jobStop := job.Start.Add(job.Duration)
		if addPadding {
			job.Start = job.Start.Add(-cfg.PrePadding)
			jobStop = jobStop.Add(postPadding)
		}
		start, err := ptypes.TimestampProto(job.Start)
		if err != nil {
			log.Printf("%d: Failed to convert Start-Timestamp: %v", job.ID, err)
		}
		stop, err := ptypes.TimestampProto(jobStop)
		if err != nil {
			log.Printf("%d: Failed to convert Stop-Timestamp: %v", job.ID, err)
		}
		playoutJob := &api.Job{
			StartAt: start,
			StopAt:  stop,
			Source:  job.Source,
			ID:      int64(job.ID),
			Version: job.Version,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		scheduledJob, err := playoutClient.SchedulePlayout(ctx, playoutJob)
		if err != nil {
			log.Printf("Failed to schedule %d: %v", job.ID, err)
			cancel()
			continue
		}

		log.Printf("Scheduled %v", job.ID)
		scheduledJob.Room = job.Room
		scheduledJobs[job.ID] = *scheduledJob
		cancel()
	}
	return scheduledJobs
}

func removeAlreadyScheduledJobs(jobs map[int]fahrplan.PlayoutJob, scheduled map[int]api.ScheduledJob) map[int]fahrplan.PlayoutJob {
	for id := range scheduled {
		if jobs[id].Version != scheduled[id].Version {
			continue
		}
		delete(jobs, id)
	}
	return jobs
}

func scheduler(cfg *Configuration, store *store.Store, upcomingChannel *bcast.Member, scheduledChannel *bcast.Member) chan struct{} {
	quit := make(chan struct{})
	go func(cfg *Configuration, upcomingChannel *bcast.Member, scheduledChannel *bcast.Member) {
		scheduled := make(map[int]api.ScheduledJob)
		for upcoming := range upcomingChannel.Read {
			u := upcoming.(map[int]fahrplan.PlayoutJob)
			if !cfg.AutoSchedule {
				continue
			}
			toSchedule := removeAlreadyScheduledJobs(u, scheduled)
			if len(toSchedule) == 0 {
				log.Println("Nothing new to Schedule")
			} else {
				scheduled = schedule(cfg, store, toSchedule, scheduled, true)
				scheduledChannel.Send(scheduled)
			}
		}
	}(cfg, upcomingChannel, scheduledChannel)
	return quit
}
