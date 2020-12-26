package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"playout-controller/fahrplan"
	"playout-controller/ingest"
	"time"
)

type Configuration struct {
	Address          string            `yaml:"Address" env:"ADDRESS"`
	FahrplanURL      string            `yaml:"FahrplanUrl" env:"FAHRPLAN_URL"`
	Fahrplanrefresh  time.Duration     `yaml:"Fahrplanrefresh" env:"FAHRPLAN_REFRESH"`
	AutoSchedule     bool              `yaml:"AutoSchedule" env:"AUTOSCHEDULE"`
	UpcomingInterval time.Duration     `yaml:"UpcomingInterval" env:"UPCOMINGINTERVAL"`
	IngestServer     IngestServer      `yaml:"IngestServer"`
	PlayoutServers   map[string]string `yaml:"PlayoutServers"`
}
type IngestServer struct {
	Nginx   []string `yaml:"nginx,omitempty"`
	Icecast []string `yaml:"icecast,omitempty"`
}

func getJobs(fahrplanURL string, version string, jobs map[int]fahrplan.PlayoutJob) string {
	schedule := new(fahrplan.Fahrplan)
	if err := fahrplan.GetSchedule(schedule, fahrplanURL); err != nil {
		log.Printf("Failed to get Fahrplan: ", err)
	}
	if schedule.Schedule.Version == version {
		log.Printf("Fahrplan version %s is still up to date\n", version)
		return version
	}
	newJobs := fahrplan.ConvertScheduleToPLayoutJobs(schedule)
	for id := range jobs{
		if _, ok := newJobs[id]; ok {
			jobs[id] = newJobs[id]
			delete(newJobs, id)
		} else {
			delete(jobs, id)
		}
	}
	for id, job := range newJobs {
		jobs[id] = job
	}
	return schedule.Schedule.Version
}

func refreshFahrplan(interval time.Duration, fahrplanURL string) (map[int]fahrplan.PlayoutJob, chan struct{}, chan bool) {
	jobs := make(map[int]fahrplan.PlayoutJob)

	ticker := time.NewTicker(interval)
	quit := make(chan struct{})
	firstJobs := make(chan bool)
	go func(fahrplanURL string, jobs map[int]fahrplan.PlayoutJob, firstJobs chan bool) {
		version := getJobs(fahrplanURL, "", jobs)
		//firstJobs <- true
		close(firstJobs)
		for {
			select {
			case <-ticker.C:
				version = getJobs(fahrplanURL, version, jobs)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(fahrplanURL, jobs, firstJobs)
	return jobs, quit, firstJobs
}

func getUpcoming(jobs map[int]fahrplan.PlayoutJob, interval time.Duration, firstJobs chan bool) (map[int]fahrplan.PlayoutJob, chan struct{}, chan bool) {
	upcoming := make(map[int]fahrplan.PlayoutJob)

	ticker := time.NewTicker(interval / 4)
	quit := make(chan struct{})
	firstUpcoming := make(chan bool)
	go func(jobs map[int]fahrplan.PlayoutJob, upcoming map[int]fahrplan.PlayoutJob, firstJobs chan bool, firstUpcoming chan bool) {
		<-firstJobs
		log.Println("Got first Jobs")
		for id, job := range fahrplan.GetUpcoming(jobs, interval){
			upcoming[id] = job
		}
		close(firstUpcoming)

		for {
			select {
			case <-ticker.C:
				u := fahrplan.GetUpcoming(jobs, interval)
				for id := range upcoming {
					if _, ok := u[id]; ok {
						upcoming[id] = u[id]
						delete(u, id)
					} else {
						delete(upcoming, id)
					}
				}
				for id, job := range u {
					upcoming[id] = job
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(jobs, upcoming, firstJobs, firstUpcoming)
	return upcoming, quit, firstUpcoming
}


func main() {
	cfg := new(Configuration)

	err := cleanenv.ReadConfig("config.yml", cfg)
	if err != nil {
		log.Fatal("Failed to load Config: ", err)
	}
	jobs, _, firstJobs := refreshFahrplan(cfg.Fahrplanrefresh, cfg.FahrplanURL)

	var sources []ingest.Source //nolint:prealloc
	for _, s := range cfg.IngestServer.Icecast {
		sources = append(sources, ingest.Source{
			Url:        s,
			IngestType: ingest.IcecastIngest,
		})
	}
	for _, s := range cfg.IngestServer.Nginx {
		sources = append(sources, ingest.Source{
			Url:        s,
			IngestType: ingest.NginxRTMPIngest,
		})
	}

	upcomingJobs, _, firstUpcoming := getUpcoming(jobs, cfg.UpcomingInterval, firstJobs)
	scheduled := make(map[int]ScheduledJob)
	scheduler(upcomingJobs, scheduled, cfg, firstUpcoming)

	log.Printf("%v\n", cfg)

	app := fiber.New()
	app.Use(cors.New())

	app.Static("/", "./static")

	api := app.Group("/api")
	api.Get("/all", func(c *fiber.Ctx) error {
		return c.JSON(jobs)
	})
	api.Get("/upcoming", func(c *fiber.Ctx) error {
		return c.JSON(upcomingJobs)
	})
	api.Get("/scheduled", func(c *fiber.Ctx) error {
		return c.JSON(scheduled)
	})
	api.Post("/schedulePlayout", func(ctx *fiber.Ctx) error {
		jobs := make(map[int]fahrplan.PlayoutJob)
		jsonErr := ctx.BodyParser(jobs)
		if jsonErr != nil {
			ctx.SendStatus(400)
			return jsonErr
		}
		scheduled = schedule(jobs, cfg.PlayoutServers, scheduled)
		ctx.JSON(scheduled)
		return nil
	})

	log.Fatal(app.Listen(cfg.Address))
}
