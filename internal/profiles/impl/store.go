// Package impl loads search profiles from YAML files on each call.
package impl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/andrewmysliuk/jobhound_core/internal/profiles"
	"github.com/andrewmysliuk/jobhound_core/internal/profiles/schema"
)

// FileStore reads config/profiles-style YAML. It does not cache file contents.
type FileStore struct {
	dir   string
	known map[string]struct{}
}

// NewFileStore returns a store that reads dir on every List and Get.
// knownSourceIDs is the only set of allowed profile sources.
func NewFileStore(dir string, knownSourceIDs []string) *FileStore {
	known := make(map[string]struct{}, len(knownSourceIDs))
	for _, id := range knownSourceIDs {
		known[id] = struct{}{}
	}
	return &FileStore{dir: dir, known: known}
}

var _ profiles.Store = (*FileStore)(nil)

type fileProfile struct {
	ID             string                      `yaml:"id"`
	Name           string                      `yaml:"name"`
	Domain         string                      `yaml:"domain"`
	Sources        []string                    `yaml:"sources"`
	Queries        []string                    `yaml:"queries"`
	Children       map[string][]string         `yaml:"children"`
	WellfoundRoles []string                    `yaml:"wellfound_roles"`
	GateRole       []string                    `yaml:"gate_role"`
	GateHard       []string                    `yaml:"gate_hard"`
	GateLocation   fileGateLocation            `yaml:"gate_location"`
	SignalsPlus    fileSignalsPlus             `yaml:"signals_plus"`
	PenaltiesSoft  map[string]filePenaltyGroup `yaml:"penalties_soft"`
	Buckets        fileBuckets                 `yaml:"buckets"`
}

type fileGateLocation struct {
	AllowCountries  []string `yaml:"allow_countries"`
	AllowRegions    []string `yaml:"allow_regions"`
	RejectTimezones []string `yaml:"reject_timezones"`
}

type fileSignalsPlus struct {
	Title     map[string]int       `yaml:"title"`
	Text      fileTextSignals      `yaml:"text"`
	Salary    fileSalarySignals    `yaml:"salary"`
	Freshness fileFreshnessSignals `yaml:"freshness"`
}

type fileTextSignals struct {
	Terms    map[string]int `yaml:"terms"`
	MaxTotal int            `yaml:"max_total"`
}

type fileSalarySignals struct {
	MinEURYear     int     `yaml:"min_eur_year"`
	MinUSDYear     int     `yaml:"min_usd_year"`
	USDPerEUR      float64 `yaml:"usd_per_eur"`
	BonusIfAbove   int     `yaml:"bonus_if_above"`
	PenaltyIfBelow int     `yaml:"penalty_if_below"`
}

type fileFreshnessSignals struct {
	Days3 int `yaml:"days_3"`
	Days7 int `yaml:"days_7"`
}

type filePenaltyGroup struct {
	Weight         int      `yaml:"weight"`
	MinOccurrences int      `yaml:"min_occurrences"`
	CheckNegation  bool     `yaml:"check_negation"`
	Terms          []string `yaml:"terms"`
}

type fileBuckets struct {
	Top int `yaml:"top"`
}

func (s *FileStore) List(ctx context.Context) ([]schema.Profile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]schema.Profile, 0)
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		id := strings.TrimSuffix(name, ".yaml")
		profile, err := s.load(filepath.Join(s.dir, name), id)
		if err != nil {
			return nil, err
		}
		out = append(out, profile)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *FileStore) Get(ctx context.Context, id string) (schema.Profile, error) {
	if err := ctx.Err(); err != nil {
		return schema.Profile{}, err
	}
	if !safeProfileID(id) {
		return schema.Profile{}, profiles.ErrProfileNotFound
	}
	return s.load(filepath.Join(s.dir, id+".yaml"), id)
}

func (s *FileStore) load(path, wantID string) (schema.Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return schema.Profile{}, profiles.ErrProfileNotFound
		}
		return schema.Profile{}, err
	}
	raw, err := decodeProfile(data)
	if err != nil {
		return schema.Profile{}, fmt.Errorf("%w: %w", profiles.ErrProfileInvalid, err)
	}
	if raw.ID != wantID {
		return schema.Profile{}, fmt.Errorf("%w: id %q does not match file", profiles.ErrProfileInvalid, raw.ID)
	}
	domain, err := schema.FromStringDomain(raw.Domain)
	if err != nil {
		return schema.Profile{}, fmt.Errorf("%w: %w", profiles.ErrProfileInvalid, err)
	}
	allowed := make(map[string]struct{}, len(raw.Sources))
	for _, sourceID := range raw.Sources {
		if _, ok := s.known[sourceID]; !ok {
			return schema.Profile{}, fmt.Errorf("%w: unknown source id %q", profiles.ErrProfileInvalid, sourceID)
		}
		allowed[sourceID] = struct{}{}
	}
	for sourceID := range raw.Children {
		if _, ok := allowed[sourceID]; !ok {
			return schema.Profile{}, fmt.Errorf("%w: children source id %q is not in sources", profiles.ErrProfileInvalid, sourceID)
		}
	}
	return schema.Profile{
		ID:             raw.ID,
		Name:           raw.Name,
		Domain:         domain,
		Sources:        raw.Sources,
		Queries:        raw.Queries,
		Children:       raw.Children,
		WellfoundRoles: raw.WellfoundRoles,
		GateRole:       raw.GateRole,
		GateHard:       raw.GateHard,
		GateLocation: schema.GateLocation{
			AllowCountries:  raw.GateLocation.AllowCountries,
			AllowRegions:    raw.GateLocation.AllowRegions,
			RejectTimezones: raw.GateLocation.RejectTimezones,
		},
		SignalsPlus: schema.SignalsPlus{
			Title: raw.SignalsPlus.Title,
			Text: schema.TextSignals{
				Terms:    raw.SignalsPlus.Text.Terms,
				MaxTotal: raw.SignalsPlus.Text.MaxTotal,
			},
			Salary: schema.SalarySignals{
				MinEURYear:     raw.SignalsPlus.Salary.MinEURYear,
				MinUSDYear:     raw.SignalsPlus.Salary.MinUSDYear,
				USDPerEUR:      raw.SignalsPlus.Salary.USDPerEUR,
				BonusIfAbove:   raw.SignalsPlus.Salary.BonusIfAbove,
				PenaltyIfBelow: raw.SignalsPlus.Salary.PenaltyIfBelow,
			},
			Freshness: schema.FreshnessSignals{
				Days3: raw.SignalsPlus.Freshness.Days3,
				Days7: raw.SignalsPlus.Freshness.Days7,
			},
		},
		PenaltiesSoft: flattenPenalties(raw.PenaltiesSoft),
		Buckets:       schema.Buckets{Top: raw.Buckets.Top},
	}, nil
}

func flattenPenalties(raw map[string]filePenaltyGroup) []schema.PenaltyGroup {
	if len(raw) == 0 {
		return nil
	}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]schema.PenaltyGroup, 0, len(names))
	for _, name := range names {
		group := raw[name]
		out = append(out, schema.PenaltyGroup{
			Name:           name,
			Weight:         group.Weight,
			MinOccurrences: group.MinOccurrences,
			CheckNegation:  group.CheckNegation,
			Terms:          group.Terms,
		})
	}
	return out
}

func decodeProfile(data []byte) (fileProfile, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var raw fileProfile
	if err := dec.Decode(&raw); err != nil {
		return fileProfile{}, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return raw, nil
		}
		return fileProfile{}, err
	}
	return fileProfile{}, errors.New("multiple yaml documents")
}

func safeProfileID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, `/\`)
}
