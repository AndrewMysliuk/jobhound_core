package wellfound

import (
	"strings"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/collectors/utils"
	"github.com/andrewmysliuk/jobhound_core/internal/domain/schema"
)

// JobDetail is parsed from a job detail page (JSON-LD JobPosting).
type JobDetail struct {
	Title           string
	Company         string
	Description     string
	PostedAt        time.Time
	Remote          *bool
	HiringCountries []string
	HiringRegions   []string
	HiringRaw       string
}

// ParseJobDetailHTML extracts JobPosting fields from detail HTML.
func ParseJobDetailHTML(html string, countries *utils.CountryResolver) (JobDetail, error) {
	jp, err := utils.FirstJobPostingFromHTML(html, "wellfound")
	if err != nil {
		return JobDetail{}, err
	}
	var out JobDetail
	out.Title = strings.TrimSpace(jp.Title)
	out.Company = strings.TrimSpace(jp.Company)
	out.Description = strings.TrimSpace(jp.Description)
	if t, err := time.Parse(time.RFC3339Nano, jp.DatePosted); err == nil {
		out.PostedAt = t.UTC()
	} else if t, err := time.Parse(time.RFC3339, jp.DatePosted); err == nil {
		out.PostedAt = t.UTC()
	}
	telecommute := strings.EqualFold(strings.TrimSpace(jp.JobLocationType), "TELECOMMUTE")
	if telecommute {
		v := true
		out.Remote = &v
	}
	locStrings := append([]string(nil), utils.LocationStringsFromJSONLD(jp.ApplicantLocationRequirements)...)
	locStrings = append(locStrings, utils.LocationStringsFromJSONLD(jp.JobLocation)...)
	out.HiringCountries, out.HiringRegions, out.HiringRaw = utils.ParseHiringScope(countries, locStrings...)
	if telecommute && len(out.HiringCountries) == 0 && len(out.HiringRegions) == 0 {
		out.HiringRegions = []string{schema.RegionCodeWorldwide.String()}
	}
	return out, nil
}
