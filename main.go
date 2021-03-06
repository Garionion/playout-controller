package main

import (
	"github.com/Garionion/playout-controller/fahrplan"
	"github.com/Garionion/playout-controller/store"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/grafov/bcast"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"net"
	"time"
)

type Configuration struct {
	Address             string            `yaml:"Address" env:"ADDRESS"`
	FahrplanURL         string            `yaml:"FahrplanUrl" env:"FAHRPLAN_URL"`
	Fahrplanrefresh     time.Duration     `yaml:"Fahrplanrefresh" env:"FAHRPLAN_REFRESH"`
	AutoSchedule        bool              `yaml:"AutoSchedule" env:"AUTOSCHEDULE"`
	UpcomingInterval    time.Duration     `yaml:"UpcomingInterval" env:"UPCOMINGINTERVAL"`
	PrePadding          time.Duration     `yaml:"PrePadding"`
	MaxPostPadding      time.Duration     `yaml:"MaxPostPadding"`
	IngestServer        IngestServer      `yaml:"IngestServer"`
	PlayoutServers      map[string]string `yaml:"PlayoutServers"`
	StudioIngestURLFile string            `yaml:"StudioIngestURLFile"`
	TalkIDtoStudioFile  string            `yaml:"TalkIDtoStudioFile"`
}
type IngestServer struct {
	Nginx   []string `yaml:"nginx,omitempty"`
	Icecast []string `yaml:"icecast,omitempty"`
}

func getJobs(fahrplanURL string, version string, talkIDtoIngestURL map[int]string) (string, map[int]fahrplan.PlayoutJob) {
	schedule := new(fahrplan.Fahrplan)
	if err := fahrplan.GetSchedule(schedule, fahrplanURL); err != nil {
		log.Printf("Failed to get Fahrplan: %v", err)
	}
	if schedule.Schedule.Version == version {
		log.Printf("Fahrplan version %s is still up to date\n", version)
	} else {
		log.Printf("NEW Fahrplan version %s", schedule.Schedule.Version)
	}
	jobs := fahrplan.ConvertScheduleToPLayoutJobs(schedule, talkIDtoIngestURL)
	return schedule.Schedule.Version, jobs
}

func refreshFahrplan(cfg *Configuration, talkIDtoIngestURL map[int]string, jobChannel *bcast.Member) {
	ticker := time.NewTicker(cfg.Fahrplanrefresh)
	quit := make(chan struct{})

	go func(fahrplanURL string) {
		version, jobs := getJobs(fahrplanURL, "", talkIDtoIngestURL)
		jobChannel.Send(jobs)
		for {
			select {
			case <-ticker.C:
				version, jobs = getJobs(fahrplanURL, version, talkIDtoIngestURL)
				jobChannel.Send(jobs)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(cfg.FahrplanURL)
}

func getUpcoming(cfg *Configuration, store *store.Store, jobChannel *bcast.Member, upcomingChannel *bcast.Member) chan struct{} {
	interval := minOfDuration(cfg.UpcomingInterval/4, cfg.Fahrplanrefresh)

	ticker := time.NewTicker(interval)
	quit := make(chan struct{})
	go func() {
		jobs := jobChannel.Recv().(map[int]fahrplan.PlayoutJob)
		upcoming := fahrplan.GetUpcoming(jobs, cfg.UpcomingInterval)
		upcomingChannel.Send(upcoming)
		for {
			select {
			case <-ticker.C:
				store.RLock()
				jobs := store.PlayoutJobs
				store.RUnlock()
				upcoming := fahrplan.GetUpcoming(jobs, cfg.UpcomingInterval)
				upcomingChannel.Send(upcoming)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	return quit
}

func minOfDuration(d1 time.Duration, d2 time.Duration) time.Duration {
	if d1 < d2 {
		return d1
	}
	return d2
}

func main() {
	cfg := new(Configuration)
	jobChannel := bcast.NewGroup()
	go jobChannel.Broadcast(0)
	upcomingChannel := bcast.NewGroup()
	go upcomingChannel.Broadcast(0)
	scheduledChannel := bcast.NewGroup()
	go scheduledChannel.Broadcast(0)
	err := cleanenv.ReadConfig("config.yml", cfg)
	if err != nil {
		log.Fatal("Failed to load Config: ", err)
	}

	s, _ := store.NewStore(jobChannel.Join(), upcomingChannel.Join(), scheduledChannel.Join(), cfg.PlayoutServers)

	talkToIngestURL := getTalkIngestURL(cfg.TalkIDtoStudioFile, cfg.StudioIngestURLFile)
	refreshFahrplan(cfg, talkToIngestURL, jobChannel.Join())

	getUpcoming(cfg, s, jobChannel.Join(), upcomingChannel.Join())
	scheduler(cfg, s, upcomingChannel.Join(), scheduledChannel.Join())

	log.Printf("%v\n", cfg)

	app := fiber.New()
	app.Use(cors.New())

	app.Static("/", "./static")

	api := app.Group("/api")
	api.Get("/all", func(c *fiber.Ctx) error {
		s.RLock()
		p := s.PlayoutJobs
		s.RUnlock()
		return c.JSON(p)
	})
	api.Get("/upcoming", func(c *fiber.Ctx) error {
		s.RLock()
		u := s.Upcoming
		s.RUnlock()
		return c.JSON(u)
	})
	api.Get("/scheduled", func(c *fiber.Ctx) error {
		s.RLock()
		scheduled := s.Scheduled
		s.RUnlock()
		return c.JSON(scheduled)
	})
	api.Post("/schedulePlayout", func(ctx *fiber.Ctx) error {
		job := new(fahrplan.PlayoutJob)
		jsonErr := json.Unmarshal(ctx.Body(), job)
		if jsonErr != nil {
			log.Println("got defective request: ", jsonErr)
			ctx.SendStatus(400)
			return jsonErr
		}
		pjobs := make(map[int]fahrplan.PlayoutJob)
		pjobs[job.ID] = *job
		s.RLock()
		scheduled := s.Scheduled
		s.RUnlock()
		newScheduled := schedule(cfg, s, pjobs, scheduled, false)
		s.SetScheduledJobs(newScheduled)
		ctx.JSON(newScheduled)
		return nil
	})
	ln, err := net.Listen("tcp", ":8080") //nolint:gosec
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Listener(ln))
}
