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
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/multi"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/rs/zerolog"
)

// MVPCollectors returns Europe Remotely, Working Nomads, Built In, and optionally Himalayas as separate collectors.
// (shared HTTP client without jar).
// Use for debug HTTP per-source routes or tests.
// httpClient may be nil (defaults from collectors/utils). dataDir is JOBHOUND_DATA_DIR semantics:
// empty uses subdirectory "data" under the current working directory for countries.json.
// himCfg: when Disabled, himalayas is nil.
// builtinCfg from config.Load().BuiltinCollector (JOBHOUND_COLLECTOR_BUILTIN_*).
// browserCfg from config.Load().Browser (JOBHOUND_BROWSER_*); Enabled defaults true unless JOBHOUND_BROWSER_ENABLED=0.
// When Enabled and builtinCfg.UseBrowserForHTML, a rod-backed fetcher is shared by Built In and Golang Cafe.
func MVPCollectors(ctx context.Context, httpClient *http.Client, dataDir string, builtinCfg config.BuiltinCollectorConfig, himCfg config.HimalayasCollectorConfig, browserCfg config.BrowserConfig) (europeRemotely, workingNomads, builtIn, himal, remotifyEurope, weWorkRemotely, wellfoundColl, vueJobs, golangCafe collectors.Collector, err error) {
	if httpClient == nil {
		httpClient = utils.NewHTTPClient()
	}
	cr, err := loadCountryResolver(dataDir)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	nonce, err := europeremotely.DiscoverNonce(ctx, httpClient)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	siteBase, err := europeremotely.DefaultSiteBase()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
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
	var htmlFetch browserfetch.HTMLDocumentFetcher
	useBuiltinBrowser := false
	if browserCfg.Enabled && builtinCfg.UseBrowserForHTML {
		f, ferr := browserfetch.NewRodFetcher(browserfetch.RodOptions{
			Bin:         browserCfg.Bin,
			UserDataDir: browserCfg.UserDataDir,
			NavTimeout:  browserCfg.NavTimeout,
			NoSandbox:   browserCfg.NoSandbox,
		})
		if ferr != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("collectors bootstrap: browser fetcher: %w", ferr)
		}
		htmlFetch = f
		useBuiltinBrowser = true
	}
	builtinColl := &builtin.BuiltIn{
		HTTPClient:          httpClient,
		InterRequestDelay:   builtinCfg.InterRequestDelay,
		HTMLDocumentFetcher: htmlFetch,
		UseBrowser:          useBuiltinBrowser,
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
	re := &remotifyeurope.RemotifyEurope{HTTPClient: httpClient}
	wwr := &weworkremotely.WeWorkRemotely{HTTPClient: httpClient, Countries: cr}
	wf := &wellfound.Wellfound{HTTPClient: httpClient}
	vj := &vuejobs.VueJobs{HTTPClient: httpClient}
	var gc collectors.Collector
	if htmlFetch != nil {
		gc = &golangcafe.GolangCafe{HTMLDocumentFetcher: htmlFetch}
	}
	return er, wn, builtinColl, him, re, wwr, wf, vj, gc, nil
}

// MVPMulti wraps MVP collectors in one collectors.Collector (DefaultIngestSourceIDs order; Himalayas when non-nil).
// Optional log: per-source Fetch failures log at Warn on multi.All when OnSourceError is unset.
func MVPMulti(
	europeRemotely, workingNomads, builtIn, himalayas,
	remotifyEurope, weWorkRemotely, wellfoundColl, vueJobs, golangCafe collectors.Collector,
	log *zerolog.Logger,
) collectors.Collector {
	list := []collectors.Collector{europeRemotely, workingNomads, builtIn}
	if himalayas != nil {
		list = append(list, himalayas)
	}
	for _, c := range []collectors.Collector{remotifyEurope, weWorkRemotely, wellfoundColl, vueJobs, golangCafe} {
		if c != nil {
			list = append(list, c)
		}
	}
	return &multi.All{Collectors: list, Log: log}
}

// MVPCollector returns a single collectors.Collector that runs all MVP sources.
func MVPCollector(ctx context.Context, httpClient *http.Client, dataDir string, builtinCfg config.BuiltinCollectorConfig, himCfg config.HimalayasCollectorConfig, browserCfg config.BrowserConfig, log *zerolog.Logger) (collectors.Collector, error) {
	er, wn, bi, h, re, wwr, wf, vj, gc, err := MVPCollectors(ctx, httpClient, dataDir, builtinCfg, himCfg, browserCfg)
	if err != nil {
		return nil, err
	}
	return MVPMulti(er, wn, bi, h, re, wwr, wf, vj, gc, log), nil
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
