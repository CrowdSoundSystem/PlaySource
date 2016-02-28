package mopidy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

var (
	idCounter      = 0
	jsonRPCVersion = "2.0"
)

type PlayState int

const (
	Unknown = iota
	Playing
	Paused
	Stopped
)

type Client struct {
	url string
}

func NewClient(url string) *Client {
	return &Client{url: url}
}

type mopidyRequest struct {
	Method  string `json:"method"`
	Version string `json:"jsonrpc"`
	ID      int    `json:"id"`

	Params interface{} `json:"params"`
}

type modipyResponse struct {
	Result      json.RawMessage `json:"result"`
	ErrorResult struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
		Data    struct {
			Message string
		} `json:"data"`
	} `json:"error"`
}

func (m *modipyResponse) Error() error {
	return fmt.Errorf("code = %v, message = %v", m.ErrorResult.Code, m.ErrorResult.Message)
}

func (c *Client) request(method string, params interface{}) (response *modipyResponse, err error) {
	body, err := json.Marshal(mopidyRequest{
		Method:  method,
		Version: jsonRPCVersion,
		ID:      idCounter,
		Params:  params,
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	response = new(modipyResponse)
	err = json.Unmarshal(b, response)
	if err != nil {
		return nil, err
	}

	if response.ErrorResult.Code != 0 {
		return nil, response.Error()
	}

	return response, nil
}

type SearchArgs struct {
	TrackName []string `json:"track_name,omitempty"`
	Artist    []string `json:"artist,omitempty"`
	Genre     []string `json:"genre,omitempty"`
}

type SearchResult struct {
	Tracks []Track `json:"tracks"`
}

func (c *Client) Search(args SearchArgs) (searchResults []SearchResult, err error) {
	resp, err := c.request("core.library.search", args)
	if err != nil {
		return searchResults, err
	}

	searchResults = make([]SearchResult, 0)
	if err = json.Unmarshal([]byte(resp.Result), &searchResults); err != nil {
		return searchResults, err
	}

	return searchResults, nil
}

func (c *Client) Play() error {
	_, err := c.request("core.playback.play", struct{}{})
	return err
}

func (c *Client) Resume() error {
	_, err := c.request("core.playback.resume", struct{}{})
	return err
}

func (c *Client) Pause() error {
	_, err := c.request("core.playback.pause", struct{}{})
	return err
}

func (c *Client) Stop() error {
	_, err := c.request("core.playback.stop", struct{}{})
	return err
}

func (c *Client) CurrentState() (playState PlayState, err error) {
	resp, err := c.request("core.playback.get_state", struct{}{})
	if err != nil {
		return Unknown, err
	}

	var state string
	err = json.Unmarshal(resp.Result, &state)
	if err != nil {
		return Unknown, err
	}

	switch state {
	case "playing":
		playState = Playing
		break
	case "stopped":
		playState = Stopped
		break
	case "paused":
		playState = Stopped
		break
	default:
		playState = Unknown
	}

	return playState, nil
}

func (c *Client) CurrentlyPlaying() (track Track, err error) {
	resp, err := c.request("core.playback.get_current_track", struct{}{})
	if err != nil {
		return track, err
	}

	err = json.Unmarshal(resp.Result, &track)
	return track, err
}

func (c *Client) History() (uris []string, err error) {
	resp, err := c.request("core.history.get_history", struct{}{})
	if err != nil {
		return uris, err
	}

	// Seriously, mopidy, why can't you play nice. This is so ugly.
	var resultSet [][]interface{}
	if err = json.Unmarshal(resp.Result, &resultSet); err != nil {
		return uris, err
	}

	uris = make([]string, 0)
	for _, r := range resultSet {
		uris = append(uris, r[1].(map[string]interface{})["uri"].(string))
	}

	return uris, err
}

func (c *Client) SetConsume(consume bool) error {
	params := struct {
		Value bool `json:"value"`
	}{
		Value: consume,
	}

	_, err := c.request("core.tracklist.set_consume", params)
	return err
}

func (c *Client) AddTracks(tracks []Track) (tracksAdded []Track, err error) {
	params := struct {
		URIs []string `json:"uris"`
	}{URIs: make([]string, len(tracks))}

	for i := range tracks {
		params.URIs[i] = tracks[i].URI
	}

	resp, err := c.request("core.tracklist.add", params)
	if err != nil {
		return tracksAdded, err
	}

	if err = json.Unmarshal(resp.Result, &tracksAdded); err != nil {
		return tracksAdded, err
	}

	return tracksAdded, err
}

func (c *Client) ClearTracklist() error {
	_, err := c.request("core.tracklist.clear", struct{}{})
	return err
}
