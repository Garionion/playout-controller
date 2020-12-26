package ingest

import (
	"encoding/xml"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sync"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var re = regexp.MustCompile(`(?mU)_([0-9]*)_`)

func getFile(u string) ([]byte, error) {
	resp, err := http.Get(u)
	//defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return nil, fmt.Errorf("Got not OK: %s", resp.Status)
	}

	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	return body, nil
}

func getIcecastSources(sources *Icecast, u string) error {
	body, err := getFile(u)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, sources); err != nil {
		return err
	}
	return nil
}

func getNginxRTMPSources(sources *RTMP, u string) error {
	body, err := getFile(u)
	if err != nil {
		return err
	}

	if err := xml.Unmarshal(body, sources); err != nil {
		return err
	}

	return nil
}

func parseIcecastSources(sources *Icestats) ([]Source, error) {
	var ingests []Source //nolint:prealloc
	for _, stream := range sources.Source {
		ingest := Source{
			Url:        stream.Listenurl,
			IngestType: IcecastIngest,
		}
		ingests = append(ingests, ingest)
	}
	return ingests, nil
}

func parseRTMPSources(source Application, u *url.URL) ([]Source, error) {

	var ingests []Source //nolint:prealloc
	appName := source.Name
	for _, stream := range source.Live {
		streamName := stream.Stream.Name
		u.Path = path.Join(u.Path, appName, streamName)
		ingest := Source{
			Url:        u.String(),
			IngestType: NginxRTMPIngest,
		}
		ingests = append(ingests, ingest)
	}
	return ingests, nil
}

func GetStreamSources(sources []Source) []Source {
	var wg sync.WaitGroup
	ingestChannel := make(chan []Source)

	var ingests []Source
	for _, source := range sources {
		switch source.IngestType {
		case IcecastIngest:
			wg.Add(1)
			go func(u string, ingestChannel chan []Source, wg *sync.WaitGroup) {
				defer wg.Done()
				var icecastSources Icecast
				p, err := url.Parse(u)
				if err != nil {
					log.Println("URL ", u, " is not valid, skipping")
					return
				}
				p.Path = path.Join(p.Path, "status-json.xsl")
				u = p.String()

				if err := getIcecastSources(&icecastSources, u); err != nil {
					log.Println("Could not get Icecast Stats: ", err)
				}
				ingest, err := parseIcecastSources(icecastSources.Icestats)
				if err != nil {
					log.Println("Could not parse Icecast Stats from ", u, err)
				}
				ingestChannel <- ingest
			}(source.Url, ingestChannel, &wg)
			break
		case NginxRTMPIngest:
			wg.Add(1)
			go func(u string, ingestChannel chan []Source, wg *sync.WaitGroup) {
				defer wg.Done()
				var rtmpSources RTMP
				p, err := url.Parse(u)
				if err != nil {
					log.Println("URL ", u, " is not valid, skipping")
					return
				}
				if err := getNginxRTMPSources(&rtmpSources, u); err != nil {
					log.Println("Could not get Nginx RTMP Stats: ", err)
				}

				for _, application := range rtmpSources.Servers.Application {
					wg.Add(1)
					go func(application Application, p *url.URL, wg *sync.WaitGroup) {
						defer wg.Done()
						ingest, err := parseRTMPSources(application, p)
						if err != nil {
							log.Println("Could not parse RTMP Stats from ", u, err)
						}
						ingestChannel <- ingest
					}(application, p, wg)
				}
			}(source.Url, ingestChannel, &wg)
			break
		}
	}
	go func(ingestChannel chan []Source, wg *sync.WaitGroup) {
		wg.Wait()
		close(ingestChannel)
	}(ingestChannel, &wg)

	for i := range ingestChannel {
		ingests = append(ingests, i...)
	}
	return ingests
}
