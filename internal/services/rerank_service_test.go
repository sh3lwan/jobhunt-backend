package services

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh3lwan/jobhunter/internal/repository"
)

func TestGeoIneligibility(t *testing.T) {
	svc := &RerankService{constraints: "Based in Cairo, Egypt; needs sponsorship for country-restricted roles"}

	match := func(restriction string, sponsorship *bool, location string) repository.GetMatchesNeedingRerankRow {
		m := repository.GetMatchesNeedingRerankRow{
			GeoRestriction: pgtype.Text{String: restriction, Valid: restriction != ""},
			Location:       pgtype.Text{String: location, Valid: location != ""},
		}
		if sponsorship != nil {
			m.GeoSponsorship = pgtype.Bool{Bool: *sponsorship, Valid: true}
		}
		return m
	}

	yes, no := true, false

	tests := []struct {
		name     string
		match    repository.GetMatchesNeedingRerankRow
		blocked  bool
		capScore int
	}{
		{"open scope stays eligible", match("", nil, "Remote"), false, 0},
		{"sponsorship offered stays eligible", match("us-only", &yes, "San Francisco"), false, 0},
		{"restriction with sponsorship refused caps at 25", match("us-only", &no, "New York"), true, 25},
		{"restriction with sponsorship unstated caps at 45", match("eu-only", nil, "Berlin"), true, 45},
		{"in-country local to candidate stays eligible", match("in-country", nil, "Cairo, Egypt"), false, 0},
		{"in-country elsewhere caps at 45", match("in-country", nil, "São Paulo, Brazil"), true, 45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capScore, note, blocked := svc.geoIneligibility(tt.match)
			if blocked != tt.blocked {
				t.Fatalf("blocked = %v, want %v", blocked, tt.blocked)
			}
			if capScore != tt.capScore {
				t.Fatalf("capScore = %d, want %d", capScore, tt.capScore)
			}
			if blocked && note == "" {
				t.Fatal("blocked match must carry a gap note")
			}
		})
	}

	unconstrained := &RerankService{}
	if _, _, blocked := unconstrained.geoIneligibility(match("us-only", &no, "New York")); blocked {
		t.Fatal("gate must be inactive when no candidate constraints are configured")
	}
}
