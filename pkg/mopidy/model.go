package mopidy

type Track struct {
	Name   string
	URI    string
	Length int

	Artists []Artist
}

type Artist struct {
	Name string
	URI  string
}
