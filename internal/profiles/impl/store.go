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
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	Domain         string   `yaml:"domain"`
	Sources        []string `yaml:"sources"`
	Queries        []string `yaml:"queries"`
	WellfoundRoles []string `yaml:"wellfound_roles"`
	ExcludeTitle   []string `yaml:"exclude_title"`
	ExcludeText    []string `yaml:"exclude_text"`
	Penalties      []string `yaml:"penalties"`
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
	for _, sourceID := range raw.Sources {
		if _, ok := s.known[sourceID]; !ok {
			return schema.Profile{}, fmt.Errorf("%w: unknown source id %q", profiles.ErrProfileInvalid, sourceID)
		}
	}
	return schema.Profile{
		ID:             raw.ID,
		Name:           raw.Name,
		Domain:         domain,
		Sources:        raw.Sources,
		Queries:        raw.Queries,
		WellfoundRoles: raw.WellfoundRoles,
		ExcludeTitle:   raw.ExcludeTitle,
		ExcludeText:    raw.ExcludeText,
		Penalties:      raw.Penalties,
	}, nil
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
