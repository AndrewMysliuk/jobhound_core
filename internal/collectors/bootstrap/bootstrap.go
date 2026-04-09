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

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dou"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/multi"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/rs/zerolog"
)

// MVPCollectors returns Europe Remotely, Working Nomads, and DOU.ua as separate collectors.
// (shared HTTP client without jar for ER/WN; DOU uses its own jar-backed client).
// Use for debug HTTP per-source routes or tests.
// httpClient may be nil (defaults from collectors/utils). dataDir is JOBHOUND_DATA_DIR semantics:
// empty uses subdirectory "data" under the current working directory for countries.json.
// douCfg comes from config.Load().DouCollector (JOBHOUND_COLLECTOR_DOU_*).
func MVPCollectors(ctx context.Context, httpClient *http.Client, dataDir string, douCfg config.DouCollectorConfig) (europeRemotely, workingNomads, douUa collectors.Collector, err error) {
	if httpClient == nil {
		httpClient = utils.NewHTTPClient()
	}
	cr, err := loadCountryResolver(dataDir)
	if err != nil {
		return nil, nil, nil, err
	}
	nonce, err := europeremotely.DiscoverNonce(ctx, httpClient)
	if err != nil {
		return nil, nil, nil, err
	}
	siteBase, err := europeremotely.DefaultSiteBase()
	if err != nil {
		return nil, nil, nil, err
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
	douColl := &dou.DOU{
		HTTPClient:        utils.NewHTTPClientWithJar(),
		Search:            douCfg.Search,
		MaxJobs:           douCfg.MaxJobsPerFetch,
		InterRequestDelay: douCfg.InterRequestDelay,
		Countries:         cr,
	}
	return er, wn, douColl, nil
}

// MVPMulti wraps MVP collectors in one collectors.Collector (order: Europe Remotely, Working Nomads, DOU.ua).
// Optional log: per-source Fetch failures log at Warn on multi.All when OnSourceError is unset.
func MVPMulti(europeRemotely, workingNomads, douUa collectors.Collector, log *zerolog.Logger) collectors.Collector {
	return &multi.All{Collectors: []collectors.Collector{europeRemotely, workingNomads, douUa}, Log: log}
}

// MVPCollector returns a single collectors.Collector that runs all MVP sources.
func MVPCollector(ctx context.Context, httpClient *http.Client, dataDir string, douCfg config.DouCollectorConfig, log *zerolog.Logger) (collectors.Collector, error) {
	er, wn, d, err := MVPCollectors(ctx, httpClient, dataDir, douCfg)
	if err != nil {
		return nil, err
	}
	return MVPMulti(er, wn, d, log), nil
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
