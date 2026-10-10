package main

import (
	"context"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
	"github.com/andrewmysliuk/jobhound_core/internal/ingest"
)

type stubCollector struct {
	name string
}

func (s stubCollector) Name() string { return s.name }

func (s stubCollector) Fetch(context.Context) ([]schema.Job, error) { return nil, nil }

func TestIngestCollectorMap_keys(t *testing.T) {
	er := stubCollector{name: europeremotely.SourceName}
	wn := stubCollector{name: workingnomads.SourceName}
	bi := stubCollector{name: builtin.SourceName}
	him := stubCollector{name: himalayas.SourceName}
	re := stubCollector{name: remotifyeurope.SourceName}
	wwr := stubCollector{name: weworkremotely.SourceName}
	wf := stubCollector{name: wellfound.SourceName}
	vj := stubCollector{name: vuejobs.SourceName}
	gc := stubCollector{name: golangcafe.SourceName}

	want := []string{
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
	m := ingestCollectorMap(er, wn, bi, him, re, wwr, wf, vj, gc)
	if len(m) != len(want) {
		t.Fatalf("worker map len=%d want %d", len(m), len(want))
	}
	for _, id := range want {
		if _, ok := m[id]; !ok {
			t.Fatalf("worker map missing %q", id)
		}
	}
	for _, dropped := range []string{"djinni", "dou_ua", "linkedin"} {
		if _, ok := m[dropped]; ok {
			t.Fatalf("worker map still has dropped source %q", dropped)
		}
	}

	noHim := ingestCollectorMap(er, wn, bi, nil, re, wwr, wf, vj, gc)
	if _, ok := noHim[ingest.NormalizeSourceID(himalayas.SourceName)]; ok {
		t.Fatal("himalayas present when collector is nil")
	}
	if _, ok := noHim[ingest.NormalizeSourceID(golangcafe.SourceName)]; !ok {
		t.Fatal("golang_cafe missing when rod fetcher collector is set")
	}

	noCafe := ingestCollectorMap(er, wn, bi, him, re, wwr, wf, vj, nil)
	if _, ok := noCafe[ingest.NormalizeSourceID(golangcafe.SourceName)]; ok {
		t.Fatal("golang_cafe present when rod fetcher is nil")
	}
	if _, ok := noCafe[ingest.NormalizeSourceID(himalayas.SourceName)]; !ok {
		t.Fatal("himalayas missing when collector is set")
	}
}
