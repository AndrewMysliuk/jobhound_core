// Package bootstrap wires MVP collectors for composition roots (cmd/agent, cmd/worker).
package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/multi"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/pipeline"
)

// MVPCollectors returns Europe Remotely and Working Nomads as separate pipeline.Collectors
// (shared HTTP client and countries). Use for debug HTTP per-source routes or tests.
// httpClient may be nil (defaults from collectors/utils). dataDir is JOBHOUND_DATA_DIR semantics:
// empty uses subdirectory "data" under the current working directory for countries.json.
func MVPCollectors(ctx context.Context, httpClient *http.Client, dataDir string) (europeRemotely, workingNomads pipeline.Collector, err error) {
	if httpClient == nil {
		httpClient = utils.NewHTTPClient()
	}
	cr, err := loadCountryResolver(dataDir)
	if err != nil {
		return nil, nil, err
	}
	nonce, err := europeremotely.DiscoverNonce(ctx, httpClient)
	if err != nil {
		return nil, nil, err
	}
	siteBase, err := europeremotely.DefaultSiteBase()
	if err != nil {
		return nil, nil, err
	}
	er := &europeremotely.EuropeRemotely{
		HTTPClient: httpClient,
		FeedURL:    europeremotely.DefaultFeedURL,
		FeedForm: url.Values{
			"action":  {"erj_ajax_search"},
			"nonce":   {nonce},
			"website": {""},
		},
		SiteBase:  siteBase,
		Countries: cr,
	}
	wn := &workingnomads.WorkingNomads{
		HTTPClient: httpClient,
		Countries:  cr,
	}
	return er, wn, nil
}

// MVPMulti wraps Europe Remotely and Working Nomads collectors in one pipeline.Collector (MVP order).
func MVPMulti(europeRemotely, workingNomads pipeline.Collector) pipeline.Collector {
	return &multi.All{Collectors: []pipeline.Collector{europeRemotely, workingNomads}}
}

// MVPCollector returns a single pipeline.Collector that runs Europe Remotely and Working Nomads.
func MVPCollector(ctx context.Context, httpClient *http.Client, dataDir string) (pipeline.Collector, error) {
	er, wn, err := MVPCollectors(ctx, httpClient, dataDir)
	if err != nil {
		return nil, err
	}
	return MVPMulti(er, wn), nil
}

func loadCountryResolver(dataDir string) (*utils.CountryResolver, error) {
	dir := strings.TrimSpace(dataDir)
	if dir == "" {
		dir = "data"
	} else {
		dir = filepath.Clean(dir)
	}
	p := filepath.Join(dir, "countries.json")
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("collectors bootstrap: countries file %q: %w", p, err)
	}
	defer f.Close()
	return utils.LoadCountryResolver(f)
}
