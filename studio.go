package main

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
)

func readData(fileName string) ([][]string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return [][]string{}, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return [][]string{}, err
	}

	return records, nil
}

func getIDtoStudio(file string) map[int]string {
	records, err := readData(file)
	if err != nil {
		log.Fatal(err)
	}

	stid := make(map[int]string)
	for _, record := range records {
		id, err := strconv.Atoi(record[0])
		if err != nil {
			log.Printf("could'nt parse %v to integer", record[0])
			continue
		}
		stid[id] = record[1]
	}
	return stid
}

func getStudioIngestURL(file string) map[string]string {
	studioIngestUrls := make(map[string]string)
	records, err := readData(file)

	if err != nil {
		log.Fatal(err)
	}
	for _, record := range records {
		studioIngestUrls[record[1]] = record[2]
	}
	return studioIngestUrls
}

func getTalkIngestURL(talkidtostudio string, studiotourl string) map[int]string {
	talks := getIDtoStudio(talkidtostudio)
	studios := getStudioIngestURL(studiotourl)
	talkIngestURLs := make(map[int]string)
	for id, studio := range talks {
		talkIngestURLs[id] = studios[studio]
	}
	return talkIngestURLs
}
