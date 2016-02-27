package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/crowdsoundsystem/playsource/pkg/playsource"
	"github.com/crowdsoundsystem/playsource/pkg/server"

	"google.golang.org/grpc"
)

var (
	mopidyUrl    = flag.String("mopidyUrl", "http://localhost:6680/mopidy/rpc", "Mopidy RPC endpoint")
	port         = flag.Int("port", 50052, "Port to listen on")
	queueSize    = flag.Int("queueSize", 3, "Anticipated client queue size")
	pollInterval = flag.Int("pollInterval", 10, "Mopidy poll time in seconds")
)

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%v", *port))
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	playsource.RegisterPlaySourceServer(
		grpcServer,
		server.NewMopidyServer(
			*mopidyUrl,
			*queueSize,
			time.Duration(*pollInterval)*time.Second,
		),
	)

	log.Println("Listenning on:", fmt.Sprintf("localhost:%v", *port))
	grpcServer.Serve(lis)
}
