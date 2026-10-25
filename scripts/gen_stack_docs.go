//go:build ignore

// Regenerates docs/STACK.md from go.mod, docker-compose.yml, Makefile, and
// .env.example. Run: make docs-stack
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		fail(fmt.Errorf("run from repo root (go.mod missing): %w", err))
	}

	goVer, requires, replaces := parseGoMod(filepath.Join(root, "go.mod"))
	images := parseComposeImages(filepath.Join(root, "docker-compose.yml"))
	dockerfileGo := parseDockerfileGo(filepath.Join(root, "Dockerfile"))
	targets := parseMakefilePhony(filepath.Join(root, "Makefile"))
	envRows := parseEnvExample(filepath.Join(root, ".env.example"))
	testCount := countTestFiles(root)
	lintVer := parseCILintVersion(filepath.Join(root, ".github/workflows/ci.yml"))
	nuxt, vue := parseFrontendVersions(filepath.Join(root, "..", "jobhound_frontend", "package.json"))

	var b bytes.Buffer
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# STACK\n\n")
	w("Generated inventory: versions, config files, make targets, env names.\n")
	w("Regenerate with `make docs-stack` (`scripts/gen_stack_docs.go`).\n")
	w("Sources: `go.mod`, `docker-compose.yml`, `Dockerfile`, `Makefile`, `.env.example`, CI workflow.\n")
	w("Do not edit by hand — change the sources or the generator.\n\n")
	w("Product narrative (handwritten): [`SYSTEM.md`](./SYSTEM.md).\n\n")

	writeSection3(&b, goVer, dockerfileGo, requires, replaces, images, lintVer, nuxt, vue)
	writeSection5(&b, targets)
	writeSection10(&b, targets, testCount, lintVer)
	writeSection11(&b, envRows, images)

	out := filepath.Join(root, "docs", "STACK.md")
	if err := os.WriteFile(out, b.Bytes(), 0o644); err != nil {
		fail(err)
	}
	fmt.Println("wrote", out)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "gen_stack_docs: %v\n", err)
	os.Exit(1)
}

func parseGoMod(path string) (goVer string, requires [][2]string, replaces []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	lines := strings.Split(string(data), "\n")
	inRequire, inReplace := false, false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "go ") {
			goVer = strings.TrimSpace(strings.TrimPrefix(trim, "go"))
			continue
		}
		if trim == "require (" {
			inRequire = true
			continue
		}
		if trim == "replace (" {
			inReplace = true
			continue
		}
		if trim == ")" {
			inRequire, inReplace = false, false
			continue
		}
		if strings.HasPrefix(trim, "require ") && !strings.HasSuffix(trim, "(") {
			parts := strings.Fields(trim)
			if len(parts) >= 3 {
				requires = append(requires, [2]string{parts[1], parts[2]})
			}
			continue
		}
		if inRequire && trim != "" && !strings.HasPrefix(trim, "//") {
			parts := strings.Fields(trim)
			if len(parts) >= 2 && !strings.Contains(trim, "// indirect") {
				requires = append(requires, [2]string{parts[0], parts[1]})
			}
		}
		if inReplace && trim != "" && !strings.HasPrefix(trim, "//") {
			replaces = append(replaces, trim)
		}
	}
	sort.Slice(requires, func(i, j int) bool { return requires[i][0] < requires[j][0] })
	return goVer, requires, replaces
}

func parseComposeImages(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	out := map[string]string{}
	re := regexp.MustCompile(`(?m)^\s+image:\s+(\S+)`)
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		img := m[1]
		switch {
		case strings.HasPrefix(img, "postgres:"):
			out["postgres"] = img
		case strings.HasPrefix(img, "redis:"):
			out["redis"] = img
		case strings.HasPrefix(img, "temporalio/auto-setup:"):
			out["temporal"] = img
		case strings.HasPrefix(img, "temporalio/ui:"):
			out["temporal_ui"] = img
		case strings.HasPrefix(img, "busybox:"):
			out["busybox"] = img
		case img == "jobhound-core:local":
			out["app"] = img
		}
	}
	return out
}

func parseDockerfileGo(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	re := regexp.MustCompile(`FROM\s+(golang:\S+)`)
	if m := re.FindSubmatch(data); len(m) == 2 {
		return string(m[1])
	}
	return ""
}

func parseMakefilePhony(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	var targets []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, ".PHONY:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, ".PHONY:"))
		for _, t := range strings.Fields(rest) {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			targets = append(targets, t)
		}
	}
	return targets
}

func parseEnvExample(path string) [][2]string {
	f, err := os.Open(path)
	if err != nil {
		fail(err)
	}
	defer f.Close()
	var rows [][2]string
	sc := bufio.NewScanner(f)
	var pendingComment string
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if trim == "" {
			pendingComment = ""
			continue
		}
		if strings.HasPrefix(trim, "#") {
			pendingComment = strings.TrimSpace(strings.TrimPrefix(trim, "#"))
			continue
		}
		eq := strings.IndexByte(trim, '=')
		if eq <= 0 {
			continue
		}
		name := trim[:eq]
		if !strings.HasPrefix(name, "JOBHOUND_") {
			continue
		}
		where := pendingComment
		if where == "" {
			where = "(see .env.example)"
		}
		rows = append(rows, [2]string{name, where})
		pendingComment = ""
	}
	if err := sc.Err(); err != nil {
		fail(err)
	}
	return rows
}

func countTestFiles(root string) int {
	n := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "bin" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			n++
		}
		return nil
	})
	return n
}

func parseCILintVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// CI installs with: ... install.sh | sh -s -- -b "..." v2.13.2
	re := regexp.MustCompile(`install\.sh[^\n]*\bv(\d+\.\d+\.\d+)\b`)
	if m := re.FindSubmatch(data); len(m) == 2 {
		return string(m[1])
	}
	re = regexp.MustCompile(`golangci-lint\s+v(\d+\.\d+\.\d+)`)
	if m := re.FindSubmatch(data); len(m) == 2 {
		return string(m[1])
	}
	return ""
}

func parseFrontendVersions(path string) (nuxt, vue string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "^4.2.1", "^3.5.25"
	}
	nuxt = jsonDep(data, `"nuxt"`)
	vue = jsonDep(data, `"vue"`)
	if nuxt == "" {
		nuxt = "^4.2.1"
	}
	if vue == "" {
		vue = "^3.5.25"
	}
	return nuxt, vue
}

func jsonDep(data []byte, key string) string {
	re := regexp.MustCompile(regexp.QuoteMeta(key) + `\s*:\s*"([^"]+)"`)
	if m := re.FindSubmatch(data); len(m) == 2 {
		return string(m[1])
	}
	return ""
}

func req(requires [][2]string, path string) string {
	for _, r := range requires {
		if r[0] == path {
			return r[1]
		}
	}
	return "?"
}

func verNum(v string) string {
	return strings.TrimPrefix(v, "v")
}

func writeSection3(b *bytes.Buffer, goVer, dockerfileGo string, requires [][2]string, replaces []string, images map[string]string, lintVer, nuxt, vue string) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	pg := images["postgres"]
	if pg == "" {
		pg = "postgres:16"
	}
	pgMajor := strings.TrimPrefix(pg, "postgres:")
	redisTag := strings.TrimPrefix(images["redis"], "redis:")
	if redisTag == "" {
		redisTag = "7-alpine"
	}
	temporal := images["temporal"]
	temporalUI := images["temporal_ui"]

	w("## 3. Stack and versions\n\n")
	w("Sources: `go.mod` / `go.sum`, `Dockerfile`, `Dockerfile.migrate`, `docker-compose.yml`, `Makefile`.\n\n")
	w("| Layer | Fact |\n")
	w("|------|------|\n")
	w("| Language | Go **%s** (`go.mod` `go %s`; Docker `%s`) |\n", goVer, goVer, dockerfileGo)
	w("| HTTP | stdlib `net/http` ServeMux (method+path patterns). No Gin/Echo/Chi. |\n")
	w("| DB | PostgreSQL **%s** + GORM **%s** + `gorm.io/driver/postgres` **%s**; pool via `pgx/v5` **%s** |\n",
		pgMajor, verNum(req(requires, "gorm.io/gorm")), verNum(req(requires, "gorm.io/driver/postgres")), verNum(req(requires, "github.com/jackc/pgx/v5")))
	w("| Migrations | `github.com/golang-migrate/migrate/v4` **%s**; SQL files in `migrations/` |\n", req(requires, "github.com/golang-migrate/migrate/v4"))
	w("| Orchestration | Temporal SDK **%s**; Compose image `%s`, UI `%s`. `go.temporal.io/api` **%s** is a direct `require`. |\n",
		req(requires, "go.temporal.io/sdk"), temporal, temporalUI, req(requires, "go.temporal.io/api"))
	w("| Cache / ingest coord | Redis **%s** + `github.com/redis/go-redis/v9` **%s** |\n", redisTag, req(requires, "github.com/redis/go-redis/v9"))
	w("| HTML | `github.com/PuerkitoBio/goquery` **%s** |\n", req(requires, "github.com/PuerkitoBio/goquery"))
	w("| Headless | `github.com/go-rod/rod` **%s** (direct `require`; used by `collectors/browserfetch`). Image installs `chromium`. |\n", req(requires, "github.com/go-rod/rod"))
	w("| Profiles | `gopkg.in/yaml.v3` **%s**. Directory from `JOBHOUND_PROFILES_DIR` (empty → `config/profiles`). |\n", req(requires, "gopkg.in/yaml.v3"))
	w("| Scoring | In-process rules in `internal/scoring/impl` (phrase gates + fact signals from ready verdicts). |\n")
	w("| Enrich | `internal/enrich`: Anthropic Message Batches over stdlib `net/http` (no Anthropic SDK). Model reply JSON Schema embedded under `enrich/schema/json_schema/`. |\n")
	w("| Text / currency | `golang.org/x/text` **%s** (NFC for enrich hashes; currency/language helpers in coerce). |\n", req(requires, "golang.org/x/text"))
	w("| Logging | `github.com/rs/zerolog` **%s** |\n", req(requires, "github.com/rs/zerolog"))
	w("| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6` **%s** (product API bodies + enrich model reply) |\n", req(requires, "github.com/santhosh-tekuri/jsonschema/v6"))
	w("| IDs | `github.com/google/uuid` **%s** |\n", req(requires, "github.com/google/uuid"))
	w("| Tests | `github.com/stretchr/testify` **%s**; `miniredis/v2` **%s**; `gorm.io/driver/sqlite` **%s** for storage unit tests |\n",
		req(requires, "github.com/stretchr/testify"),
		req(requires, "github.com/alicebob/miniredis/v2"),
		req(requires, "gorm.io/driver/sqlite"))
	w("| Lint / format | golangci-lint **v2** (`.golangci.yml`: `errcheck`, `govet`, `staticcheck`, `unused`, `errorlint`, `depguard`; formatter `gofmt`). `make lint` / `make fmt` / `make vet`. CI installs **v%s**. No Prettier/ESLint in this repo. |\n", lintVer)
	w("| UI | Sibling `jobhound_frontend`: Nuxt **%s**, Vue **%s**. |\n\n", nuxt, vue)

	w("### direct `require` (`go.mod`)\n\n")
	w("| Package | Version |\n")
	w("|---------|---------|\n")
	for _, r := range requires {
		w("| `%s` | %s |\n", r[0], r[1])
	}
	w("\n")
	if len(replaces) > 0 {
		w("`replace`: ")
		parts := make([]string, 0, len(replaces))
		for _, r := range replaces {
			parts = append(parts, "`"+r+"`")
		}
		w("%s.\n\n", strings.Join(parts, "; "))
	}
}

func writeSection5(b *bytes.Buffer, targets []string) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	has := func(name string) bool {
		for _, t := range targets {
			if t == name {
				return true
			}
		}
		return false
	}
	_ = has

	w("## 5. Configuration\n\n")
	w("| File | What is configured |\n")
	w("|------|----------------|\n")
	w("| `go.mod` | module path, Go version, direct deps, `replace` pins |\n")
	w("| `Makefile` | `build` five app binaries to `bin/` (agent, worker, api, migrate, retention); `bin/ipv6proxy` is a separate target used by `start-ipv6-proxy`; `eval` runs `go run ./cmd/eval`; `run` / `run-debug` / `run-worker`; `test` / `test-integration`; `fmt` / `vet` / `lint` (`golangci-lint run`) / `tidy`; migrate; `db-dump` / `db-restore` (full Postgres dump under gitignored `data/local/`, restore then migrate); `docker-up` starts the host proxy, then Compose `build --no-cache` + `up -d --force-recreate --pull always`; `docker-down` stops the proxy; `docs-stack` regenerates `docs/STACK.md` |\n")
	w("| `docker-compose.yml` | postgres 16, migrate one-shot, redis 7, temporal + UI + temporal-ready, agent :3001, worker, api :3000. Worker and agent get `JOBHOUND_EUROPE_REMOTELY_PROXY=http://host.docker.internal:18080`. `api` and `worker` get `JOBHOUND_PROFILES_DIR=/app/config/profiles` and optional `JOBHOUND_ANTHROPIC_API_KEY` / `JOBHOUND_ENRICH_MODEL`. The proxy process itself is not a Compose service |\n")
	w("| `Dockerfile` | multi-stage; CGO_ENABLED=0; debian-slim + chromium + `data/` + `config/profiles` |\n")
	w("| `Dockerfile.migrate` | migrate binary + `migrations/` |\n")
	w("| `.dockerignore` | `.git`, `bin/`, `.cursor`, `*.md`, `.env*` |\n")
	w("| `.gitignore` | `.env`, `.env.*`, `/bin/`, `vendor/`, sqlite, `cookies.txt`, dump path under `data/local/` |\n")
	w("| `.cursor/rules/specify-rules.mdc` | always-applied stack + module layout, Canonical Enum, schema-first HTTP, Temporal separation |\n")
	w("| `.cursor/templates/` | concept / schemas-contracts / tasks for new `docs/<feature_slug>/` slices |\n")
	w("| `.cursor/skills/audit-feature-tasks/` | audit `*-tasks.md` against code |\n")
	w("| `.cursor/skills/calibrate-profile/` | calibrate YAML/scorer from pasted TOP vacancies |\n")
	w("| CI | `.github/workflows/ci.yml` on push and pull request: `gofmt -l` empty, `make vet`, `make lint`, `make test`. No integration tag. No deploy |\n")
	w("| `.env.example` | every key `internal/config` loads; secrets empty; default in a comment. README links it and does not paste the table |\n")
	w("| `commitlint` / OpenAPI / CSS | none |\n")
	w("| `.editorconfig` | `utf-8`, `lf`, final newline, trim trailing space. `*.go` tabs. `*.{yml,yaml,json,md}` indent 2. `*.md` does not trim trailing space |\n")
	w("| LICENSE | **none** |\n\n")
	w("Non-standard: env names live only in `internal/config`. Feature packages must not `os.Getenv` shared knobs. Temporal **connection** is config; workflow **code** is per-module `workflows/`.\n\n")
}

func writeSection10(b *bytes.Buffer, targets []string, testCount int, lintVer string) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }

	purpose := map[string]string{
		"build":              "`bin/agent`, `bin/worker`, `bin/api`, `bin/migrate`, `bin/retention` (`bin/ipv6proxy` and `cmd/eval` are not part of `build`)",
		"run":                "build + run agent (loads `.env` if present)",
		"run-debug":          "agent with `JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:3001`",
		"run-worker":         "Temporal worker",
		"eval":               "`go run ./cmd/eval` (default `testdata/enrich/gold.jsonl` + `predictions.jsonl`)",
		"test":               "`go test ./...` (excludes `integration` tag)",
		"test-integration":   "`go test -tags=integration ./...`",
		"fmt":                "`go fmt ./...`",
		"vet":                "`go vet ./...`",
		"lint":               "`golangci-lint run`",
		"tidy":               "`go mod tidy`",
		"migrate-up":         "`bin/migrate up`",
		"migrate-down":       "`bin/migrate down`",
		"migrate-version":    "`bin/migrate version`",
		"db-dump":            "Full `pg_dump` via `JOBHOUND_DATABASE_URL` into gitignored `data/local/`",
		"db-restore":         "Restore that dump, then run migrations",
		"start-ipv6-proxy":   "Build `bin/ipv6proxy` if needed; listen on `127.0.0.1:18080` (`-detach`, pid in `bin/ipv6proxy.pid`, log `bin/ipv6proxy.log`). Health check is `GET /health`",
		"stop-ipv6-proxy":    "Stop host ipv6 proxy via `bin/ipv6proxy.pid`",
		"docker-up":          "Compose. Depends on `start-ipv6-proxy`",
		"docker-down":        "Compose down; runs `stop-ipv6-proxy` first",
		"docker-down-volumes": "Alias of `docker-down`",
		"docker-ps":          "Compose `ps`",
		"docker-logs":        "Compose `logs -f`",
		"docker-migrate":     "Compose `run --rm migrate`",
		"docs-stack":         "Regenerate `docs/STACK.md` from go.mod / Compose / Makefile / `.env.example`",
		"build-retention":    "Alias of `build`",
	}

	w("## 10. Build, tests, CI\n\n")
	w("### Makefile\n\n")
	w("| Target | Purpose |\n")
	w("|--------|---------|\n")
	// Stable display order for the main table; append any unknown phony at the end.
	order := []string{
		"build", "run", "run-debug", "run-worker", "eval",
		"test", "test-integration", "fmt", "vet", "lint", "tidy",
		"migrate-up", "migrate-down", "migrate-version",
		"db-dump", "db-restore",
		"start-ipv6-proxy", "stop-ipv6-proxy",
		"docker-up", "docker-down", "docker-ps", "docker-logs", "docker-migrate",
		"docs-stack",
	}
	listed := map[string]struct{}{}
	for _, t := range order {
		p, ok := purpose[t]
		if !ok {
			continue
		}
		inPhony := false
		for _, ph := range targets {
			if ph == t {
				inPhony = true
				break
			}
		}
		if !inPhony && t != "docs-stack" {
			// Still document canonical targets even if .PHONY line wraps oddly.
		}
		_ = inPhony
		w("| `%s` | %s |\n", t, p)
		listed[t] = struct{}{}
	}
	// Compact fmt/vet/lint/tidy was one row historically; keep individual rows from order.
	for _, t := range targets {
		if _, ok := listed[t]; ok {
			continue
		}
		p := purpose[t]
		if p == "" {
			p = "(see Makefile)"
		}
		w("| `%s` | %s |\n", t, p)
	}
	w("\n")

	w("### Tests\n\n")
	w("- **Runner:** `go test`. **%s** `*_test.go` files.\n", strconv.Itoa(testCount))
	w("- **Unit (default):** handlers (`httptest`) for the five profile routes, `profiles/impl` YAML load (including `lada.yaml`, `children`, and `facts`), `scoring/impl` phrase + fact tables, `scoring/storage` / `jobs/storage` / `enrich/storage` (often sqlite), enrich batches/coerce/decode/eval, collector parse fixtures (software and architecture boards), ingest coordinator (miniredis).\n")
	w("- **Integration (`//go:build integration`):** `internal/platform/pgsql/migrations_integration_test.go`; `internal/ingest/coordinator_integration_test.go`; `internal/collectors/browserfetch/rod_integration_test.go`. Need env / Compose (`JOBHOUND_DATABASE_URL`, Chromium as applicable).\n")
	w("- **`tests/`:** empty directory (constitution allows optional `tests/integration/`). Offline enrich fixtures live under `testdata/enrich/`.\n")
	w("- **Frontend unit tests:** live in `jobhound_frontend` (`pnpm test:unit`), not in this repo.\n\n")

	w("### CI\n\n")
	w("`.github/workflows/ci.yml` on push and pull request (`ubuntu-latest`, Go from `go.mod`): `gofmt -l` must be empty, then `make vet`, `make lint` (golangci-lint **v%s**), `make test`. No `integration` tag. No deploy job.\n\n", lintVer)
}

func writeSection11(b *bytes.Buffer, envRows [][2]string, images map[string]string) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	_ = images

	w("## 11. Deploy\n\n")
	w("- **Production target (constitution):** GCP runtime + secrets. **No** Cloud Run workflow, Dockerfile deploy target, or Artifact Registry config is in this repo yet.\n")
	w("- **Local / current runtime:** Docker Compose on a laptop (`make docker-up`). Published ports: **5432** Postgres, **6379** Redis, **7233** Temporal gRPC, **8088** Temporal UI, **3000** API, **3001** agent debug HTTP. Host proxy **127.0.0.1:18080** is not a Compose port.\n")
	w("- **Images:** `jobhound-core:local` (agent/worker/api); migrate uses `Dockerfile.migrate`. Chromium in the app image for Built In, Golang Cafe, and the architecture rod boards `ejobs`, `dezeen_jobs`, and `world_architects` (`JOBHOUND_BROWSER_NO_SANDBOX=1` as root). `config/profiles` is in the app image.\n")
	w("- **CORS defaults:** `http://localhost:5173`, `http://localhost:3002` (frontend dev servers — not the API listen address).\n\n")

	w("### Environment variable names\n\n")
	w("From `internal/config` (single source). Values not listed. Rows below are taken from `.env.example` comments.\n\n")
	w("| Name | Where / default |\n")
	w("|------|-----------------|\n")
	for _, row := range envRows {
		w("| `%s` | %s |\n", row[0], row[1])
	}
	w("\n")
	w("`.env.example`: present. Lists every key in the table above; secrets empty; default in a comment. `README.md` links that file and does not paste a second list. Compose interpolates `JOBHOUND_API_CORS_ORIGINS`, `JOBHOUND_ANTHROPIC_API_KEY`, and `JOBHOUND_ENRICH_MODEL` from host env.\n")
}
