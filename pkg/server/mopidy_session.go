package server

import (
	"log"
	"time"

	"github.com/crowdsoundsystem/playsource/pkg/mopidy"
	"github.com/crowdsoundsystem/playsource/pkg/playsource"
)

type SongTrackPair struct {
	Song  playsource.Song
	Track mopidy.Track
}

type MopidySession struct {
	client       *mopidy.Client
	pollInterval time.Duration

	shutdown chan struct{}
	queue    chan SongTrackPair
	finished chan SongTrackPair
}

func NewMopidySession(client *mopidy.Client, queueSize int, pollInterval time.Duration) (*MopidySession, error) {
	// First, reset mopidy into a blank state.
	if err := client.SetConsume(true); err != nil {
		return nil, err
	}

	if err := client.ClearTracklist(); err != nil {
		return nil, err
	}

	if err := client.Stop(); err != nil {
		return nil, err
	}

	// Get initial history length
	history, err := client.History()
	if err != nil {
		return nil, err
	}

	session := &MopidySession{
		client:   client,
		shutdown: make(chan struct{}),
		queue:    make(chan SongTrackPair, queueSize),
		finished: make(chan SongTrackPair, queueSize),
	}

	go session.monitor(len(history))

	return session, nil
}

func (m *MopidySession) Close() error {
	close(m.shutdown)
	return nil
}

func (m *MopidySession) QueueSong(song SongTrackPair) error {
	select {
	case <-m.shutdown:
		return nil
	case m.queue <- song:
		return nil
	}
}

func (m *MopidySession) FinishedChan() <-chan SongTrackPair {
	return m.finished
}

func (m *MopidySession) monitor(initialHistoryCount int) {
	// A song enters history once it starts playing, so the count starts at 1.
	currentSize := initialHistoryCount + 1
	for {
		select {
		case <-m.shutdown:
			return
		case <-time.After(m.pollInterval):
			history, err := m.client.History()
			if err != nil {
				log.Println("Error retrieving history:", err)
				continue
			}

			newSize := len(history)
			if newSize < currentSize {
				continue
			} else if newSize == currentSize {
				state, err := m.client.CurrentState()
				if err != nil {
					log.Println("[session] Error getting state:", err)
					continue
				}

				if state == mopidy.Stopped {
					m.finished <- <-m.queue
					currentSize++
				}

				continue
			}

			for i := currentSize; i < newSize; i++ {
				m.finished <- <-m.queue
			}

			currentSize = newSize
		}
	}
}
