package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"golang.org/x/net/context"

	"github.com/crowdsoundsystem/playsource/pkg/playsource"
	"google.golang.org/grpc"
)

var (
	host      = flag.String("hostname", "localhost", "Hostname of the service")
	port      = flag.Int("port", 50052, "Port of the service")
	file      = flag.String("file", "sample_queue.json", "File containing queue of songs")
	queueSize = flag.Int("queueSize", 3, "Number of songs to be queued")
)

type Song struct {
	Name    string   `json:"name"`
	Artists []string `json:"artists"`
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.Parse()

	file, err := ioutil.ReadFile(*file)
	checkErr(err)

	var songs []Song
	checkErr(json.Unmarshal(file, &songs))

	// Specifically don't set grpc.WithTimeout(),
	// as it messes with the QueueSong() streams.
	conn, err := grpc.Dial(
		fmt.Sprintf("%v:%v", *host, *port),
		grpc.WithInsecure(),
	)
	checkErr(err)
	defer conn.Close()

	c := playsource.NewPlaySourceClient(conn)
	stream, err := c.QueueSong(context.Background())
	checkErr(err)

	// Try to play all the songs listed in the queue file using
	// the proper protocol. Note: doesn't handle retries.
	var inFlight int
	for i := 0; i < len(songs); {
		for inFlight < *queueSize {
			log.Println("Queueing song:", songs[i])
			err := stream.Send(&playsource.QueueSongRequest{
				Song: &playsource.Song{
					SongId:  int32(i),
					Name:    songs[i].Name,
					Artists: songs[i].Artists,
				},
			})
			checkErr(err)

			i++
			inFlight++
		}

		resp, err := stream.Recv()
		checkErr(err)

		inFlight--
		if inFlight < 0 {
			log.Fatalf("Negative in flight requests")
		}

		if resp.Finished {
			log.Println("Finished:", songs[resp.SongId])
		} else if !resp.Found {
			log.Println("Not Found:", songs[resp.SongId])
		} else if !resp.Queued {
			log.Println("Not Queued (possible overqueue?):", songs[resp.SongId])
		}
	}
}
