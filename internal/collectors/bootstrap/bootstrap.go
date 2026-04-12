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
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/djinni"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dou"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/multi"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/rs/zerolog"
)

// MVPCollectors returns Europe Remotely, Working Nomads, DOU.ua, Djinni, Built In, and optionally Himalayas as separate collectors.
// (shared HTTP client without jar for ER/WN/Himalayas/Djinni; DOU uses its own jar-backed client).
// Use for debug HTTP per-source routes or tests.
// httpClient may be nil (defaults from collectors/utils). dataDir is JOBHOUND_DATA_DIR semantics:
// empty uses subdirectory "data" under the current working directory for countries.json.
// douCfg comes from config.Load().DouCollector (JOBHOUND_COLLECTOR_DOU_*).
// djinniCfg from config.Load().DjinniCollector (JOBHOUND_COLLECTOR_DJINNI_*).
// himCfg: when Disabled, himalayas is nil.
// builtinCfg from config.Load().BuiltinCollector (JOBHOUND_COLLECTOR_BUILTIN_*).
func MVPCollectors(ctx context.Context, httpClient *http.Client, dataDir string, douCfg config.DouCollectorConfig, djinniCfg config.DjinniCollectorConfig, builtinCfg config.BuiltinCollectorConfig, himCfg config.HimalayasCollectorConfig) (europeRemotely, workingNomads, douUa, djin, builtIn collectors.Collector, himal collectors.Collector, err error) {
	if httpClient == nil {
		httpClient = utils.NewHTTPClient()
	}
	cr, err := loadCountryResolver(dataDir)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	nonce, err := europeremotely.DiscoverNonce(ctx, httpClient)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	siteBase, err := europeremotely.DefaultSiteBase()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
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
	djinColl := &djinni.Djinni{
		HTTPClient:        httpClient,
		MaxJobs:           djinniCfg.MaxJobsPerFetch,
		InterRequestDelay: djinniCfg.InterRequestDelay,
		Countries:         cr,
	}
	builtinColl := &builtin.BuiltIn{
		HTTPClient:        httpClient,
		InterRequestDelay: builtinCfg.InterRequestDelay,
	}
	var him *himalayas.Himalayas
	if !himCfg.Disabled {
		him = &himalayas.Himalayas{
			HTTPClient: httpClient,
			Countries:  cr,
			MaxPages:   himCfg.MaxPages,
		}
		if q := strings.TrimSpace(himCfg.Search); q != "" {
			him.UseSearch = true
			him.SearchQuery = q
			him.SearchStartPage = 1
		}
	}
	return er, wn, douColl, djinColl, builtinColl, him, nil
}

// MVPMulti wraps MVP collectors in one collectors.Collector (order: Europe Remotely, Working Nomads, DOU.ua, Djinni, Built In, Himalayas when non-nil).
// Optional log: per-source Fetch failures log at Warn on multi.All when OnSourceError is unset.
func MVPMulti(europeRemotely, workingNomads, douUa, djinni, builtIn, himalayas collectors.Collector, log *zerolog.Logger) collectors.Collector {
	list := []collectors.Collector{europeRemotely, workingNomads, douUa, djinni, builtIn}
	if himalayas != nil {
		list = append(list, himalayas)
	}
	return &multi.All{Collectors: list, Log: log}
}

// MVPCollector returns a single collectors.Collector that runs all MVP sources.
func MVPCollector(ctx context.Context, httpClient *http.Client, dataDir string, douCfg config.DouCollectorConfig, djinniCfg config.DjinniCollectorConfig, builtinCfg config.BuiltinCollectorConfig, himCfg config.HimalayasCollectorConfig, log *zerolog.Logger) (collectors.Collector, error) {
	er, wn, d, dj, bi, h, err := MVPCollectors(ctx, httpClient, dataDir, douCfg, djinniCfg, builtinCfg, himCfg)
	if err != nil {
		return nil, err
	}
	return MVPMulti(er, wn, d, dj, bi, h, log), nil
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
