package impl

import (
	"reflect"
	"testing"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/schema"
)

func TestCoerceModelReply(t *testing.T) {
	t.Parallel()

	quote := func(s string) *string { return &s }
	vac := VacancyMatchText(
		"Backend Engineer",
		"Work from anywhere. Based in Bangalore. Authorized to work in the United States. Visa sponsorship available. 25 LPA. Remote. Full time. Contract.",
		"Remote - United States",
		"25 LPA",
	)

	allUnknown := schema.ModelReply{
		Office:            unknownOffice(),
		HiringScope:       unknownHiringScope(),
		WorkAuthorization: unknownWorkAuthorization(),
		Timezone:          unknownTimezone(),
		Compensation:      unknownCompensation(),
		WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
		EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
		Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
	}

	tests := []struct {
		name string
		in   schema.ModelReply
		vac  string
		want schema.ModelReply
	}{
		{
			name: "silence stays unknown on every field",
			in:   allUnknown,
			vac:  vac,
			want: allUnknown,
		},
		{
			name: "failed quote becomes unknown",
			in: schema.ModelReply{
				Office: schema.Office{
					Countries: []string{"IN"},
					Regions:   []string{},
					Quote:     quote("not in the vacancy"),
					Unknown:   false,
				},
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: "remote", Quote: quote("Remote")},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: "remote", Quote: quote("Remote")},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "non-ISO country dropped; restricted with none left becomes unknown",
			in: schema.ModelReply{
				Office: unknownOffice(),
				HiringScope: schema.HiringScope{
					Value:     schema.ScopeRestricted.String(),
					Countries: []string{"XX", "USA"},
					Regions:   []string{"NOT_A_REGION"},
					Quote:     quote("Remote - United States"),
				},
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "non-ISO currency dropped",
			in: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation: schema.Compensation{
					Currency: "NOTISO",
					Quote:    quote("25 LPA"),
					Unknown:  false,
				},
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "zone outside enum dropped; restricted with none left becomes unknown",
			in: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone: schema.TimezoneConstraint{
					Value: schema.ScopeRestricted.String(),
					Zones: []string{"NOT_A_ZONE", "UTC+5"},
					Quote: quote("Work from anywhere"),
				},
				Compensation:   unknownCompensation(),
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "authorization required with valid quote and empty countries stays required",
			in: schema.ModelReply{
				Office:      unknownOffice(),
				HiringScope: unknownHiringScope(),
				WorkAuthorization: schema.WorkAuthorization{
					Value:     schema.AuthorizationRequired.String(),
					Countries: []string{},
					Quote:     quote("Authorized to work in the United States"),
				},
				Timezone:       unknownTimezone(),
				Compensation:   unknownCompensation(),
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:      unknownOffice(),
				HiringScope: unknownHiringScope(),
				WorkAuthorization: schema.WorkAuthorization{
					Value:     schema.AuthorizationRequired.String(),
					Countries: []string{},
					Quote:     quote("Authorized to work in the United States"),
				},
				Timezone:       unknownTimezone(),
				Compensation:   unknownCompensation(),
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "currency does not fill office",
			in: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation: schema.Compensation{
					Currency: "INR",
					Quote:    quote("25 LPA"),
					Unknown:  false,
				},
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation: schema.Compensation{
					Currency: "INR",
					Quote:    quote("25 LPA"),
					Unknown:  false,
				},
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "posting language does not fill office",
			in:   allUnknown,
			vac: VacancyMatchText(
				"Softwareentwickler",
				"Wir suchen eine erfahrene Person. Remote möglich.",
				"",
				"",
			),
			want: allUnknown,
		},
		{
			name: "badge alone does not set hiring_scope when model left unknown",
			in:   allUnknown,
			vac: VacancyMatchText(
				"Engineer",
				"Join our team.",
				"Remote - United States",
				"",
			),
			want: allUnknown,
		},
		{
			name: "visa sponsorship available stays none when model answered none",
			in: schema.ModelReply{
				Office:      unknownOffice(),
				HiringScope: unknownHiringScope(),
				WorkAuthorization: schema.WorkAuthorization{
					Value:     schema.AuthorizationNone.String(),
					Countries: []string{},
					Quote:     quote("Visa sponsorship available"),
				},
				Timezone:       unknownTimezone(),
				Compensation:   unknownCompensation(),
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:      unknownOffice(),
				HiringScope: unknownHiringScope(),
				WorkAuthorization: schema.WorkAuthorization{
					Value:     schema.AuthorizationNone.String(),
					Countries: []string{},
					Quote:     quote("Visa sponsorship available"),
				},
				Timezone:       unknownTimezone(),
				Compensation:   unknownCompensation(),
				WorkMode:       schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad: schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:     schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "valid office keeps ISO country and drops bad region",
			in: schema.ModelReply{
				Office: schema.Office{
					Countries: []string{"in", "XX"},
					Regions:   []string{"LATAM", "BOGUS"},
					Quote:     quote("Bangalore"),
					Unknown:   false,
				},
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office: schema.Office{
					Countries: []string{"IN"},
					Regions:   []string{"LATAM"},
					Quote:     quote("Bangalore"),
					Unknown:   false,
				},
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
		{
			name: "model unknown stays unknown even with a quote present",
			in: schema.ModelReply{
				Office: unknownOffice(),
				HiringScope: schema.HiringScope{
					Value:     unknownAnswer,
					Countries: []string{"US"},
					Regions:   []string{},
					Quote:     quote("Remote - United States"),
				},
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: quote("Remote")},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
			vac: vac,
			want: schema.ModelReply{
				Office:            unknownOffice(),
				HiringScope:       unknownHiringScope(),
				WorkAuthorization: unknownWorkAuthorization(),
				Timezone:          unknownTimezone(),
				Compensation:      unknownCompensation(),
				WorkMode:          schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				EmploymentLoad:    schema.QuotedValue{Value: unknownAnswer, Quote: nil},
				Engagement:        schema.QuotedValue{Value: unknownAnswer, Quote: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CoerceModelReply(tt.in, tt.vac)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v\nwant %#v", got, tt.want)
			}
		})
	}
}
