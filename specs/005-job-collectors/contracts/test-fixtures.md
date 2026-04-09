# Reference samples for offline tests (MVP)

**Spec**: `005-job-collectors`

This file holds **minimal synthetic samples** as fenced blocks — **no separate JSON/HTML files** in the spec tree. Implementation may copy these into `internal/collectors/.../testdata/` for `httptest`.

**What to assert in tests** (which cases, `httptest`, injectable clock for Europe): **`../tasks.md`** sections D.2 and E.2 — same pattern as **`specs/004-pipeline-stages/tasks.md`**.

**Normalized output:** per **`domain-mapping-mvp.md`**, HTML bodies in fixtures are **transport shape**; **`Job.Description`** in tests should assert **plain text** after stripping.

---

## Europe Remotely — `admin-ajax`-style envelope

Shape per `../resources/europe-remotely.md` (`has_more` + HTML fragment):

```json
{
  "has_more": false,
  "html": "<div class=\"job-card\"><h2 class=\"job-title\"><a href=\"https://euremotejobs.com/job/example-slug/\">Senior Go Engineer</a></h2><div class=\"company-name\">Acme EU</div><div class=\"meta-item meta-location\">Germany, Remote</div><div class=\"meta-item meta-type\">Full Time</div><div class=\"job-time\">Posted 2 days ago</div></div>"
}
```

---

## Europe Remotely — job detail HTML (excerpt)

Selectors per `../resources/europe-remotely.md`:

```html
<div class="page-header">
  <h1 class="page-title">Senior Go Engineer</h1>
  <h3 class="page-subtitle">
    <ul class="job-listing-meta meta">
      <li class="job-type">Full Time</li>
      <li class="location"><a class="google_map_link" href="https://www.google.com/maps?q=Berlin">Berlin, Germany</a></li>
      <li class="date-posted">March 28, 2026</li>
      <li class="job-company"><a href="https://euremotejobs.com/company/acme/">Acme EU</a></li>
      <li class="wpjmef-field-salary">Salary: 90,000–110,000 EUR/year</li>
    </ul>
  </h3>
</div>
<div class="job_listing-description"><p>Build distributed systems.</p></div>
<p class="job_tags">Tagged as: golang, backend, remote</p>
<a class="application_button_link" href="https://ats.example.com/apply/1">Apply for job</a>
```

---

## Working Nomads — `_search` response (one hit)

Shape per `../resources/working-nomads.md`:

```json
{
  "took": 4,
  "timed_out": false,
  "hits": {
    "total": { "value": 1, "relation": "eq" },
    "hits": [
      {
        "_index": "jobsapi",
        "_id": "1502763",
        "_score": 12.5,
        "_source": {
          "id": 1502763,
          "title": "Senior Full Stack Developer",
          "slug": "senior-full-stack-developer-acme-1502763",
          "company": "Acme Remote",
          "category_name": "Development",
          "description": "<p>Ship features end-to-end.</p>",
          "position_type": "ft",
          "tags": ["javascript", "react"],
          "locations": ["European Union"],
          "location_base": "EU",
          "pub_date": "2026-03-28T10:00:00Z",
          "apply_option": "with_your_ats",
          "apply_url": "https://example.com/apply",
          "expired": false,
          "salary_range": "€70k – €90k",
          "experience_level": "SENIOR_LEVEL"
        }
      }
    ]
  }
}
```

---

## DOU.ua — listing HTML (excerpt)

Hidden CSRF + one `li.l-vacancy` per `../resources/dou.md`:

```html
<!doctype html><html><body>
<input type="hidden" name="csrfmiddlewaretoken" value="fixturecsrf">
<li class="l-vacancy">
  <div class="date">9 квітня</div>
  <div class="title">
    <a class="vt" href="https://jobs.dou.ua/companies/acme/vacancies/353313/">Go Engineer</a>
    <strong><a class="company" href="https://jobs.dou.ua/companies/acme/">Acme UA</a></strong>
  </div>
  <span class="cities">Україна</span>
</li>
</body></html>
```

---

## DOU.ua — `xhr-load` JSON

```json
{
  "html": "<li class=\"l-vacancy\"><div class=\"date\">7 квітня</div><div class=\"title\"><a class=\"vt\" href=\"https://jobs.dou.ua/companies/other/vacancies/353314/\">Second Role</a><strong><a class=\"company\" href=\"https://jobs.dou.ua/companies/other/\">Other Co</a></strong></div><span class=\"cities\">Україна</span></li>",
  "last": true,
  "num": 2
}
```

---

## DOU.ua — job detail HTML (excerpt)

```html
<!doctype html><html><body>
<h1 class="g-h2">Go Engineer</h1>
<div class="date">8 квітня 2026</div>
<div class="sh-info">
  <span class="place">Київ, Україна</span>
  <span class="salary">до $5000</span>
</div>
<div class="b-typo vacancy-section"><p>Remote golang work.</p></div>
<a class="badge" href="https://jobs.dou.ua/vacancies/?category=backend">Backend</a>
</body></html>
```

---

## Related

- `../resources/europe-remotely.md`, `../resources/working-nomads.md`, `../resources/dou.md`
- `collector.md`
