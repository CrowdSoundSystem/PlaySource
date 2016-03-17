package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/crowdsoundsystem/playsource/pkg/playsource"
	"github.com/crowdsoundsystem/playsource/pkg/server"
	"github.com/crowdsoundsystem/playsource/pkg/systemd"

	"google.golang.org/grpc"
)

var (
	configPath   = flag.String("config", "", "Configuration path")
	mopidyUrl    = flag.String("mopidyUrl", "http://localhost:6680/mopidy/rpc", "Mopidy RPC endpoint")
	port         = flag.Int("port", 50052, "Port to listen on")
	queueSize    = flag.Int("queueSize", 200, "Anticipated client queue size")
	pollInterval = flag.Int("pollInterval", 10, "Mopidy poll time in seconds")
	test         = flag.Bool("test", false, "Whether or not to emulate a real server")
	serviceMode  = flag.Bool("serviceMode", false, "Whether or not the playsource is being run as a systemd service")
)

type Config struct {
	MopidyURL    string `json:"mopidy_url"`
	Port         int    `json:"port"`
	QueueSize    int    `json:"queue_size"`
	PollInterval int    `json:"poll_interval"`
	Test         bool   `json:"test"`
}

func loadConfig() Config {
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

	return config
}

func waitForMopidy(config Config) {
	for {
		_, err := http.Get(config.MopidyURL)
		if err == nil {
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {
	flag.Parse()

	config := loadConfig()

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
				1.1,
				120*time.Second,
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

	if *serviceMode {
		log.Println("Waiting for mopidy...")
		waitForMopidy(config)
		systemd.Ready()
	}

	log.Println("Listening on:", fmt.Sprintf("localhost:%v", config.Port))
	grpcServer.Serve(lis)
}
