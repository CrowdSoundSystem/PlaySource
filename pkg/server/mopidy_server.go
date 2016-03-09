package server

import (
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/codes"

	"golang.org/x/net/context"

	"github.com/crowdsoundsystem/playsource/pkg/mopidy"
	"github.com/crowdsoundsystem/playsource/pkg/playsource"
)

type MopidyServer struct {
	client *mopidy.Client

	queueSize    int32
	maxQueueSize int
	pollInterval time.Duration

	nowPlayingLock sync.Mutex
	nowPlaying     playsource.Song

	historyLock sync.Mutex
	history     playsource.Song

	// Buffered channel of size one. When a client connects, they
	// attempt to obtain a 'lease' from this channel. If they receive
	// a value, they are considered master, and no other client can
	// control the server (though they may query it). When master
	// disconnects, they return their 'lease' to this channel
	master   chan struct{}
	skipSong chan struct{}
}

func NewMopidyServer(url string, maxQueueSize int, pollInterval time.Duration) *MopidyServer {
	s := &MopidyServer{
		client:       mopidy.NewClient(url),
		maxQueueSize: maxQueueSize,
		pollInterval: pollInterval,
		master:       make(chan struct{}, 1),
	}

	// Initial lease
	s.master <- struct{}{}

	log.Println("created")
	return s
}

func queueStream(stream playsource.Playsource_QueueSongServer) <-chan playsource.QueueSongRequest {
	inbound := make(chan playsource.QueueSongRequest)

	go func() {
		defer close(inbound)

		for {
			req, err := stream.Recv()
			if err == io.EOF {
				log.Println("stream closed")
				return
			} else if err != nil {
				log.Println(err)
				return
			}

			log.Println("Received queue song:", req)
			inbound <- *req
		}
	}()

	return inbound
}

func (m *MopidyServer) QueueSong(stream playsource.Playsource_QueueSongServer) error {
	log.Println("Client connected")

	select {
	case <-m.master:
	default:
		return errf(codes.Unavailable, "A master already exists")
	}

	defer func() { m.master <- struct{}{} }()

	atomic.StoreInt32(&m.queueSize, 0)
	inbound := queueStream(stream)
	session, err := NewMopidySession(m.client, 2*m.maxQueueSize, m.pollInterval)
	if err != nil {
		return err
	}
	defer session.Close()

	log.Println("Starting loop")
	for {
		select {
		case req, ok := <-inbound:
			if !ok {
				// Inbound channel was closed, which means the stream was closed.
				return nil
			}

			// Search for song
			args := mopidy.SearchArgs{
				TrackName: []string{req.Song.Name},
				Artist:    req.Song.Artists,
			}

			searchResults, err := m.client.Search(args)
			if err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}

			tracks := make([]mopidy.Track, 0)
			for _, r := range searchResults {
				tracks = append(tracks, r.Tracks...)
			}

			// Did we finy any results?
			if len(tracks) == 0 {
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

			// Check server queue size.
			if int(atomic.LoadInt32(&m.queueSize)) >= m.maxQueueSize {
				log.Println("Internal queue size reached: ", atomic.LoadInt32(&m.queueSize))
				atomic.AddInt32(&m.queueSize, -1)
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

			// Just take the first result?
			tracksAdded, err := m.client.AddTracks(tracks[0:1])
			if err != nil {
				return err
			}

			if len(tracksAdded) == 0 {
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

			log.Println("Queuing:", req.Song)
			err = session.QueueSong(SongTrackPair{
				Song:  *req.Song,
				Track: tracks[0],
			})
			if err != nil {
				return err
			}

			// If we aren't playing (for whatever reason), make sure we play.
			state, err := m.client.CurrentState()
			if err != nil {
				return err
			}

			switch state {
			case mopidy.Stopped:
				err = m.client.Play()
				if err != nil {
					return err
				}
				break
			case mopidy.Paused:
				err = m.client.Resume()
				if err != nil {
					return err
				}
				break
			}
			atomic.AddInt32(&m.queueSize, 1)
		case song := <-session.FinishedChan():
			log.Println("finished:", song)
			err := stream.Send(&playsource.QueueSongResponse{
				SongId:   song.Song.SongId,
				Finished: true,
				Found:    true,
			})
			if err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}

			atomic.AddInt32(&m.queueSize, -1)
		}
	}
	return nil
}

func (m *MopidyServer) SkipSong(ctx context.Context, req *playsource.SkipSongRequest) (*playsource.SkipSongResponse, error) {
	err := m.client.Next()
	if err != nil {
		err = errf(codes.Internal, err.Error())
	}

	return &playsource.SkipSongResponse{}, err
}

func (m *MopidyServer) GetPlaying(ctx context.Context, req *playsource.GetPlayingRequest) (*playsource.GetPlayingResponse, error) {
	m.nowPlayingLock.Lock()
	song := m.nowPlaying
	m.nowPlayingLock.Unlock()

	return &playsource.GetPlayingResponse{Song: &song}, nil
}

func (m *MopidyServer) GetPlayHistory(req *playsource.GetPlayHistoryRequest, stream playsource.Playsource_GetPlayHistoryServer) error {
	/*
		m.historyLock.Lock()
		history := make([]playsource.Song, len(m.history))
		copy(history, m.history)
		m.historyLock.Unlock()

		for i := range history {
			resp := &playsource.GetPlayHistoryResponse{
				Song: &history[i],
			}

			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	*/

	return nil
}
