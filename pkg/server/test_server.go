package server

import (
	"io"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/crowdsoundsystem/playsource/pkg/playsource"
)

var errf = grpc.Errorf // Used to stop `go vet` errors for grpc.Errof

// TestServer provides the semantics of an actual playsource, without
// actually requiring one.
type TestServer struct {
	maxQueueSize     int
	songLength       time.Duration
	foundProbability float64

	nowPlayingLock sync.Mutex
	nowPlaying     playsource.Song

	master    chan struct{}
	queue     chan playsource.Song
	finished  chan playsource.Song
	shutdown  chan struct{}
	queueSize int32

	historyLock sync.Mutex
	history     []playsource.Song
}

func NewTestServer(maxQueueSize int, foundProbability float64, songLength time.Duration) *TestServer {
	t := &TestServer{
		maxQueueSize:     maxQueueSize,
		foundProbability: foundProbability,
		songLength:       songLength,
		master:           make(chan struct{}, 1),
		queue:            make(chan playsource.Song, maxQueueSize),
		finished:         make(chan playsource.Song, maxQueueSize),
	}

	t.master <- struct{}{}

	// Launch queue processor
	go func() {
		for {
			select {
			case <-t.shutdown:
				return
			case song := <-t.queue:
				t.nowPlayingLock.Lock()
				t.nowPlaying = song
				t.nowPlayingLock.Unlock()

				time.Sleep(t.songLength)
				atomic.AddInt32(&t.queueSize, -1)

				t.historyLock.Lock()
				t.history = append(t.history, song)
				t.historyLock.Unlock()

				t.finished <- song
			}
		}
	}()

	return t
}

func (t *TestServer) Close() error {
	close(t.shutdown)
	return nil
}

func (t *TestServer) QueueSong(stream playsource.Playsource_QueueSongServer) error {
	inbound := queueStream(stream)

	for {
		select {
		case req, ok := <-inbound:
			if !ok {
				return nil
			}

			// Perform lookup immediately
			if rand.Float64() < t.foundProbability {
				err := stream.Send(&playsource.QueueSongResponse{
					SongId: req.Song.SongId,
					Queued: false,
					Found:  false,
				})
				if err == io.EOF {
					return nil
				} else if err != nil {
					return err
				}

				continue
			}

			// Check queue size.
			if int(atomic.LoadInt32(&t.queueSize)) >= t.maxQueueSize {
				log.Println("Exceeded queue")
				err := stream.Send(&playsource.QueueSongResponse{
					SongId: req.Song.SongId,
					Queued: false,
					Found:  true,
				})
				if err == io.EOF {
					return nil
				} else if err != nil {
					return err
				}
			}

			// All good, go for the queue
			t.queue <- *req.Song
		case song := <-t.finished:
			log.Println("Sending back")
			err := stream.Send(&playsource.QueueSongResponse{
				SongId:   song.SongId,
				Finished: true,
				Found:    true,
			})
			if err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}
		}
	}
}

func (t *TestServer) GetPlaying(ctx context.Context, req *playsource.GetPlayingRequest) (*playsource.GetPlayingResponse, error) {
	t.nowPlayingLock.Lock()
	song := t.nowPlaying
	t.nowPlayingLock.Unlock()

	return &playsource.GetPlayingResponse{Song: &song}, nil
}

func (t *TestServer) GetPlayHistory(req *playsource.GetPlayHistoryRequest, stream playsource.Playsource_GetPlayHistoryServer) error {
	t.historyLock.Lock()
	history := make([]playsource.Song, len(t.history))
	copy(history, t.history)
	t.historyLock.Unlock()

	for i := range history {
		resp := &playsource.GetPlayHistoryResponse{
			Song: &history[i],
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}
