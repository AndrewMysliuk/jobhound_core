package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
	slotsutils "github.com/andrewmysliuk/jobhound_core/internal/slots/utils"
)

type stubCollector struct {
	name string
}

func (s stubCollector) Name() string { return s.name }

func (s stubCollector) Fetch(context.Context) ([]schema.Job, error) { return nil, nil }

func TestIngestCollectorMap_matchesDefaultIngestSourceIDs(t *testing.T) {
	wantOrder := []string{
		ingest.NormalizeSourceID(europeremotely.SourceName),
		ingest.NormalizeSourceID(workingnomads.SourceName),
		ingest.NormalizeSourceID(builtin.SourceName),
		ingest.NormalizeSourceID(himalayas.SourceName),
		ingest.NormalizeSourceID(remotifyeurope.SourceName),
		ingest.NormalizeSourceID(weworkremotely.SourceName),
		ingest.NormalizeSourceID(wellfound.SourceName),
		ingest.NormalizeSourceID(vuejobs.SourceName),
		ingest.NormalizeSourceID(golangcafe.SourceName),
	}
	ids := slotsutils.DefaultIngestSourceIDs()
	if !reflect.DeepEqual(ids, wantOrder) {
		t.Fatalf("DefaultIngestSourceIDs order %+v want %+v", ids, wantOrder)
	}

	m := ingestCollectorMap(
		stubCollector{name: europeremotely.SourceName},
		stubCollector{name: workingnomads.SourceName},
		stubCollector{name: builtin.SourceName},
		stubCollector{name: himalayas.SourceName},
		stubCollector{name: remotifyeurope.SourceName},
		stubCollector{name: weworkremotely.SourceName},
		stubCollector{name: wellfound.SourceName},
		stubCollector{name: vuejobs.SourceName},
		stubCollector{name: golangcafe.SourceName},
	)
	if len(m) != len(ids) {
		t.Fatalf("worker map len=%d DefaultIngestSourceIDs len=%d", len(m), len(ids))
	}
	for i, id := range ids {
		if _, ok := m[id]; !ok {
			t.Fatalf("index %d: worker map missing %q", i, id)
		}
	}
	for _, dropped := range []string{"djinni", "dou_ua", "linkedin"} {
		if _, ok := m[dropped]; ok {
			t.Fatalf("worker map still has dropped source %q", dropped)
		}
		for _, id := range ids {
			if id == dropped {
				t.Fatalf("DefaultIngestSourceIDs still has dropped source %q", dropped)
			}
		}
	}
}
