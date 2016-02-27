package server

import (
	"log"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/crowdsoundsystem/playsource/pkg/playsource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestQueueLoop(t *testing.T) {
	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)

	grpcServer := grpc.NewServer()
	playsource.RegisterPlaySourceServer(
		grpcServer,
		NewTestServer(10, 0.8, 3*time.Second),
	)

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	assert.NoError(t, err)
	defer conn.Close()

	c := playsource.NewPlaySourceClient(conn)

	// The general approach here is that we can keep queueing up to
	// a certain amount of songs (bounded by service). However, as a
	// client, we want to perform a local bound of how many songs we
	// queue, or have 'in-flight'. That is, we must track queue acks
	// and finished acks to determine when to send queue requests.
	// This is very similar to the networks algorithm. Funny how things
	// actually work.
	stream, err := c.QueueSong(context.Background())
	assert.NoError(t, err)

	var inFlight int

	for i := 0; i < 100; {
		for inFlight < 3 {
			log.Println("Sending request: ", i)
			err := stream.Send(&playsource.QueueSongRequest{
				Song: &playsource.Song{
					SongId:  int32(i),
					Name:    strconv.Itoa(i),
					Artists: []string{"a", "b", "c"},
				},
			})
			require.NoError(t, err)
			i++
			inFlight++
		}

		resp, err := stream.Recv()
		require.NoError(t, err)

		inFlight--
		if resp.Finished {
			log.Printf("Song %v finished playing", resp.SongId)
		} else if !resp.Found {
			log.Println("could not find song:", resp.SongId)
		}
	}
}
