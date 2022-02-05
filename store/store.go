package store

import (
	"github.com/Garionion/ffmpeg-playout/api"
	"github.com/grafov/bcast"
	"github.com/Garionion/playout-controller/fahrplan"
	"google.golang.org/grpc"
	"log"
	"sync"
)

type Store struct {
	PlayoutJobs map[int]fahrplan.PlayoutJob
	Upcoming map[int]fahrplan.PlayoutJob
	Scheduled map[int]api.ScheduledJob
	GrpcClients map[string]api.PlayoutClient
	sync.RWMutex
}

func NewStore(jobChannel *bcast.Member, upcomingChannel *bcast.Member, scheduleChannel *bcast.Member, playoutServers map[string]string) (*Store, error) {
	store := &Store{
		PlayoutJobs: map[int]fahrplan.PlayoutJob{},
		Upcoming: map[int]fahrplan.PlayoutJob{},
		Scheduled: map[int]api.ScheduledJob{},
		GrpcClients: map[string]api.PlayoutClient{},
	}
	go func(jobChannel *bcast.Member, upcomingChannel *bcast.Member, scheduleChannel *bcast.Member) {
		for  {
			select {
			case playoutJobs := <-jobChannel.Read:
				p := playoutJobs.(map[int]fahrplan.PlayoutJob)
				store.SetPlayoutJobs(p)
			case upcoming := <-upcomingChannel.Read:
				u := upcoming.(map[int]fahrplan.PlayoutJob)
				store.SetUpcomingJobs(u)
			case scheduled := <-scheduleChannel.Read:
				s := scheduled.(map[int]api.ScheduledJob)
				store.SetScheduledJobs(s)
			}
		}
	}(jobChannel, upcomingChannel , scheduleChannel )
	for roomName, address := range playoutServers{
		conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		store.GrpcClients[roomName] = api.NewPlayoutClient(conn)
	}
	return store, nil
}

func (s *Store) SetPlayoutJobs(playoutJobs map[int]fahrplan.PlayoutJob)  {
	s.Lock()
	s.PlayoutJobs = playoutJobs
	s.Unlock()
}

func (s *Store) SetUpcomingJobs(upcomingJobs map[int]fahrplan.PlayoutJob)  {
	s.Lock()
	s.Upcoming = upcomingJobs
	s.Unlock()
}

func (s *Store) SetScheduledJobs(scheduledJobs map[int]api.ScheduledJob)  {
	s.Lock()
	s.Scheduled = scheduledJobs
	s.Unlock()
}
