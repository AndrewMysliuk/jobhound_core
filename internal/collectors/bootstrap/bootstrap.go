// Package bootstrap wires MVP collectors for composition roots (cmd/agent, cmd/worker).
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/archgee"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/archinect"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/baunetz"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bdla"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/bestjobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/browserfetch"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/builtin"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/competitionline"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/consider"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/dezeen"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/ejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/europeremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/getro"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/golangcafe"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/himalayas"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/hipo"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/landscapeinstitute"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/remotifyeurope"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/riba"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/vuejobs"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/wellfound"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/weworkremotely"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/workingnomads"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/worldarchitects"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
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
// considerBoards and getroBoards are one collector per Tenants() row, sharing httpClient and the country resolver.
func MVPCollectors(ctx context.Context, httpClient *http.Client, dataDir string, builtinCfg config.BuiltinCollectorConfig, himCfg config.HimalayasCollectorConfig, browserCfg config.BrowserConfig, erCfg config.EuropeRemotelyConfig) (europeRemotely, workingNomads, builtIn, himal, remotifyEurope, weWorkRemotely, wellfoundColl, vueJobs, golangCafe, bestJobs, hipoColl, ejobsColl, dezeenColl, ribaColl, landscapeColl, archgeeColl, archinectColl, competitionlineColl, baunetzColl, bdlaColl, worldArchitectsColl collectors.Collector, considerBoards, getroBoards []collectors.Collector, err error) {
	if httpClient == nil {
		httpClient = utils.NewHTTPClient()
	}
	cr, err := loadCountryResolver(dataDir)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	erHTTP := httpClient
	if proxy := strings.TrimSpace(erCfg.ProxyURL); proxy != "" {
		c, perr := utils.HTTPClientWithProxy(httpClient, proxy)
		if perr != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("collectors bootstrap: europe remotely proxy: %w", perr)
		}
		erHTTP = c
	}
	nonce, nonceErr := europeremotely.DiscoverNonce(ctx, erHTTP)
	if nonceErr != nil {
		slog.Warn("europe remotely nonce discovery failed; that source will error on fetch", "error", nonceErr)
	}
	siteBase, err := europeremotely.DefaultSiteBase()
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	feedForm := url.Values{
		"action":  {"erj_ajax_search"},
		"website": {""},
	}
	if nonceErr == nil {
		feedForm.Set("nonce", nonce)
	}
	er := &europeremotely.EuropeRemotely{
		HTTPClient: erHTTP,
		FeedURL:    europeremotely.DefaultFeedURL,
		FeedForm:   feedForm,
		SiteBase:   siteBase,
		Countries:  cr,
		StartErr:   nonceErr,
	}
	wn := &workingnomads.WorkingNomads{
		HTTPClient: httpClient,
		Countries:  cr,
	}
	var rodFetch *browserfetch.RodFetcher
	if browserCfg.Enabled {
		f, ferr := browserfetch.NewRodFetcher(browserfetch.RodOptions{
			Bin:         browserCfg.Bin,
			UserDataDir: browserCfg.UserDataDir,
			NavTimeout:  browserCfg.NavTimeout,
			NoSandbox:   browserCfg.NoSandbox,
		})
		if ferr != nil {
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("collectors bootstrap: browser fetcher: %w", ferr)
		}
		rodFetch = f
	}
	var htmlFetch browserfetch.HTMLDocumentFetcher
	useBuiltinBrowser := rodFetch != nil && builtinCfg.UseBrowserForHTML
	if useBuiltinBrowser {
		htmlFetch = rodFetch
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
	re := &remotifyeurope.RemotifyEurope{HTTPClient: httpClient, Countries: cr}
	wwr := &weworkremotely.WeWorkRemotely{HTTPClient: httpClient, Countries: cr}
	wf := &wellfound.Wellfound{HTTPClient: httpClient, Countries: cr}
	vj := &vuejobs.VueJobs{HTTPClient: httpClient}
	var gc collectors.Collector
	if htmlFetch != nil {
		gc = &golangcafe.GolangCafe{HTMLDocumentFetcher: htmlFetch, Countries: cr}
	}
	considerRows := consider.Tenants()
	considerColls := make([]collectors.Collector, 0, len(considerRows))
	for _, tenant := range considerRows {
		considerColls = append(considerColls, &consider.Board{
			HTTPClient: httpClient,
			Countries:  cr,
			Tenant:     tenant,
		})
	}
	getroRows := getro.Tenants()
	getroColls := make([]collectors.Collector, 0, len(getroRows))
	for _, tenant := range getroRows {
		getroColls = append(getroColls, &getro.Board{
			HTTPClient: httpClient,
			Countries:  cr,
			Tenant:     tenant,
		})
	}
	bj := &bestjobs.BestJobs{HTTPClient: httpClient, Countries: cr}
	hp := &hipo.Hipo{HTTPClient: httpClient}
	ej := &ejobs.Ejobs{}
	dz := &dezeen.Dezeen{}
	rb := &riba.Riba{HTTPClient: httpClient}
	li := &landscapeinstitute.LandscapeInstitute{HTTPClient: httpClient}
	ag := &archgee.Archgee{HTTPClient: httpClient}
	an := &archinect.Archinect{HTTPClient: httpClient}
	cl := &competitionline.Competitionline{HTTPClient: httpClient}
	bn := &baunetz.Baunetz{HTTPClient: httpClient}
	bd := &bdla.Bdla{HTTPClient: httpClient}
	wa := &worldarchitects.WorldArchitects{}
	if rodFetch != nil {
		ej.Fetcher = rodFetch
		dz.Fetcher = rodFetch
		wa.Fetcher = rodFetch
	}
	return er, wn, builtinColl, him, re, wwr, wf, vj, gc, bj, hp, ej, dz, rb, li, ag, an, cl, bn, bd, wa, considerColls, getroColls, nil
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
	defer func() { _ = f.Close() }()
	return utils.LoadCountryResolver(f)
}
