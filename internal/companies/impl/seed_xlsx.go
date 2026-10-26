package impl

import (
	"fmt"
	"strings"

	"github.com/andrewmysliuk/jobhound_core/internal/companies/schema"
	"github.com/xuri/excelize/v2"
)

const (
	seedSheetName = "Companies"

	colCompany      = "Компания"
	colWebsite      = "Сайт"
	colATS          = "ATS-тип"
	colSlug         = "ATS slug"
	colFit          = "Fit (FIT/SKIP)"
	colVacanciesURL = "URL vacancies"
)

// ReadSeedXLSX loads seed rows from companies_template.xlsx (or a fixture).
// It does not skip example / Fit=SKIP — ImportFromSeed does that.
func ReadSeedXLSX(path string) ([]schema.SeedRow, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open seed xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheet := seedSheetName
	if idx, err := f.GetSheetIndex(sheet); err != nil || idx < 0 {
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, fmt.Errorf("seed xlsx has no sheets")
		}
		sheet = sheets[0]
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read seed sheet %q: %w", sheet, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("seed sheet %q is empty", sheet)
	}

	idx, err := mapSeedColumns(rows[0])
	if err != nil {
		return nil, err
	}

	out := make([]schema.SeedRow, 0, len(rows)-1)
	for _, raw := range rows[1:] {
		if rowEmpty(raw) {
			continue
		}
		out = append(out, schema.SeedRow{
			Name:         cellAt(raw, idx.company),
			Website:      cellAt(raw, idx.website),
			ATSRaw:       cellAt(raw, idx.ats),
			Slug:         cellAt(raw, idx.slug),
			VacanciesURL: cellAt(raw, idx.vacanciesURL),
			Fit:          cellAt(raw, idx.fit),
		})
	}
	return out, nil
}

type seedColIndex struct {
	company, website, ats, slug, fit, vacanciesURL int
}

func mapSeedColumns(header []string) (seedColIndex, error) {
	idx := seedColIndex{
		company: -1, website: -1, ats: -1, slug: -1, fit: -1, vacanciesURL: -1,
	}
	for i, h := range header {
		switch strings.TrimSpace(h) {
		case colCompany:
			idx.company = i
		case colWebsite:
			idx.website = i
		case colATS:
			idx.ats = i
		case colSlug:
			idx.slug = i
		case colFit:
			idx.fit = i
		case colVacanciesURL:
			idx.vacanciesURL = i
		}
	}
	if idx.company < 0 || idx.ats < 0 || idx.fit < 0 {
		return seedColIndex{}, fmt.Errorf("seed xlsx missing required columns (%s, %s, %s)", colCompany, colATS, colFit)
	}
	return idx, nil
}

func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func rowEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
