package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/crowdsoundsystem/playsource/pkg/playsource"
	"github.com/crowdsoundsystem/playsource/pkg/server"

	"google.golang.org/grpc"
)

var (
	configPath   = flag.String("config", "", "Configuration path")
	mopidyUrl    = flag.String("mopidyUrl", "http://localhost:6680/mopidy/rpc", "Mopidy RPC endpoint")
	port         = flag.Int("port", 50052, "Port to listen on")
	queueSize    = flag.Int("queueSize", 3, "Anticipated client queue size")
	pollInterval = flag.Int("pollInterval", 10, "Mopidy poll time in seconds")
	test         = flag.Bool("test", false, "Whether or not to emulate a real server")
)

type Config struct {
	MopidyURL    string `json:"mopidy_url"`
	Port         int    `json:"port"`
	QueueSize    int    `json:"queue_size"`
	PollInterval int    `json:"poll_interval"`
	Test         bool   `json:"test"`
}

func main() {
	flag.Parse()

	var config Config
	if *configPath != "" {
		f, err := os.Open(*configPath)
		if err != nil {
			log.Fatal(err)
		}

		d := json.NewDecoder(f)
		err = d.Decode(&config)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		config.MopidyURL = *mopidyUrl
		config.Port = *port
		config.QueueSize = *queueSize
		config.PollInterval = *pollInterval
		config.Test = *test
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%v", config.Port))
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()

	if *test {
		playsource.RegisterPlaysourceServer(
			grpcServer,
			server.NewTestServer(
				config.QueueSize,
				0.9,
				5*time.Second,
			),
		)
	} else {
		playsource.RegisterPlaysourceServer(
			grpcServer,
			server.NewMopidyServer(
				config.MopidyURL,
				config.QueueSize,
				time.Duration(config.PollInterval)*time.Second,
			),
		)
	}

	log.Println("Listenning on:", fmt.Sprintf("localhost:%v", config.Port))
	grpcServer.Serve(lis)
}
