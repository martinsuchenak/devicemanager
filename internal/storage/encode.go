package storage

import (
	"encoding/json"
	"io"
)

// saveJSON saves data as JSON
func saveJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// loadJSON loads data from JSON
func loadJSON(r io.Reader, data interface{}) error {
	decoder := json.NewDecoder(r)
	return decoder.Decode(data)
}
