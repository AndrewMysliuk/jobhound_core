// Command companies-import seeds the companies table from companies_template.xlsx
// with board verify for Greenhouse / Lever / Ashby slugs.
//
// Usage:
//
//	companies-import
//	companies-import -file path/to/sheet.xlsx
//
// Requires JOBHOUND_DATABASE_URL. Reads JOBHOUND_COMPANIES_DIR for the blocklist
// (default config/companies). The seed file defaults to
// <JOBHOUND_COMPANIES_DIR>/companies_template.xlsx.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/ats/ashby"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/ats/greenhouse"
	"github.com/andrewmysliuk/jobhound_core/internal/collectors/ats/lever"
	"github.com/andrewmysliuk/jobhound_core/internal/companies"
	companiesimpl "github.com/andrewmysliuk/jobhound_core/internal/companies/impl"
	companiesstorage "github.com/andrewmysliuk/jobhound_core/internal/companies/storage"
	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/andrewmysliuk/jobhound_core/internal/platform/pgsql"
)

func main() {
	fileFlag := flag.String("file", "", "path to seed xlsx (default: <JOBHOUND_COMPANIES_DIR>/companies_template.xlsx)")
	flag.Parse()

	if err := run(*fileFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(fileArg string) error {
	appCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if appCfg.Database.URL == "" {
		return fmt.Errorf("%s is required", config.EnvDatabaseURL)
	}

	seedPath := fileArg
	if seedPath == "" {
		seedPath = filepath.Join(appCfg.Companies.Dir, "companies_template.xlsx")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	gdb, err := pgsql.Open(ctx, appCfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		sqlDB, err := gdb.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}()

	blocklist, err := companiesimpl.LoadBlocklist(appCfg.Companies.Dir)
	if err != nil {
		return err
	}

	repo := companiesstorage.NewRepository(pgsql.NewGetter(gdb))
	fetchers := []companies.CompanyBoardFetcher{
		&greenhouse.Client{},
		&lever.Client{},
		&ashby.Client{},
	}
	svc := companiesimpl.New(repo, nil, blocklist, fetchers, appCfg.Companies)

	fmt.Printf("importing %s\n", seedPath)
	stats, err := svc.ImportFromSeed(ctx, seedPath)
	if err != nil {
		return err
	}
	fmt.Printf(
		"done: rows=%d skipped=%d blocked=%d created=%d merged=%d resolved=%d pending=%d unsupported=%d\n",
		stats.Rows, stats.Skipped, stats.Blocked, stats.Created, stats.Merged,
		stats.Resolved, stats.Pending, stats.Unsupported,
	)
	return nil
}
