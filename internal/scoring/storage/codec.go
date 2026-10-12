package storage

import (
	"encoding/json"

	jobdata "github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	jobutils "github.com/andrewmysliuk/jobhound_core/internal/jobs/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/scoring/schema"
)

type locationJSON struct {
	Type      string   `json:"type"`
	Regions   []string `json:"regions"`
	Countries []string `json:"countries"`
	Timezone  string   `json:"timezone"`
	Raw       string   `json:"raw"`
}

type openingJSON struct {
	Country  string `json:"country"`
	URL      string `json:"url"`
	ApplyURL string `json:"apply_url"`
}

func encodeSignals(signals []schema.Signal) ([]byte, error) {
	if signals == nil {
		signals = []schema.Signal{}
	}
	return json.Marshal(signals)
}

func decodeSignals(b []byte) ([]schema.Signal, error) {
	if len(b) == 0 {
		return []schema.Signal{}, nil
	}
	var signals []schema.Signal
	if err := json.Unmarshal(b, &signals); err != nil {
		return nil, err
	}
	if signals == nil {
		return []schema.Signal{}, nil
	}
	return signals, nil
}

func decodeLocation(b []byte) (jobdata.Location, error) {
	if len(b) == 0 {
		return jobdata.Location{}, nil
	}
	var payload locationJSON
	if err := json.Unmarshal(b, &payload); err != nil {
		return jobdata.Location{}, err
	}
	return jobdata.Location{
		Type:      payload.Type,
		Regions:   payload.Regions,
		Countries: payload.Countries,
		Timezone:  payload.Timezone,
		Raw:       payload.Raw,
	}, nil
}

func decodeOpenings(b []byte) ([]jobdata.Opening, error) {
	if len(b) == 0 {
		return []jobdata.Opening{}, nil
	}
	var payload []openingJSON
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return []jobdata.Opening{}, nil
	}
	out := make([]jobdata.Opening, len(payload))
	for i, item := range payload {
		out[i] = jobdata.Opening{
			Country:  item.Country,
			URL:      item.URL,
			ApplyURL: item.ApplyURL,
		}
	}
	return out, nil
}

func decodeSources(b []byte) ([]string, error) {
	sources, err := jobutils.ParseSourceIDs(string(b))
	if err != nil {
		return nil, err
	}
	if sources == nil {
		return []string{}, nil
	}
	return sources, nil
}
