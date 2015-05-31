package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

type Config struct {
	Region        string
	InstanceCount int64
	IsDownload    bool
	IsServe       bool
	Filename      string
}

func main() {
	var config Config

	flag.BoolVar(&config.IsServe, "serve", false, "Start server.")
	flag.BoolVar(&config.IsDownload, "download", false, "Retrieve latest data.")
	flag.Int64Var(&config.InstanceCount, "instances", 100, "Number of running instances.")
	flag.StringVar(&config.Region, "region", "eu-west-1", "AWS region to map.")
	flag.StringVar(&config.Filename, "filename", "region.json", "Storage location of JSON files.")

	flag.Parse()

	var region *AwsRegion
	var err error

	if config.IsDownload {
		region, err = fetchRegion(&config)
		if err != nil {
			log.Fatal(err)
		}

		f, err := os.OpenFile(config.Filename, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}

		enc := json.NewEncoder(f)
		err = enc.Encode(region)
		if err != nil {
			log.Fatal(err)
		}
	}

	if config.IsServe {
		f, err := os.Open(config.Filename)
		if err != nil {
			log.Fatal(err)
		}

		dec := json.NewDecoder(f)
		err = dec.Decode(&region)
		if err != nil {
			log.Fatal(err)
		}
	}
}
