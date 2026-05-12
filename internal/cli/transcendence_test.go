package cli

import (
	"testing"
	"time"

	"cloudbeds-pp-cli/internal/store"
)

func TestParseDurationLike(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 14 * 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"2w", 14 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"30m", 30 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseDurationLike(tc.in, 14*24*time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("parseDurationLike(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseDurationLikeInvalid(t *testing.T) {
	if _, err := parseDurationLike("nonsense", 24*time.Hour); err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestComputeRateDriftEmpty(t *testing.T) {
	if got := computeRateDrift(nil); len(got) != 0 {
		t.Errorf("expected empty drift for nil input, got %v", got)
	}
}

func TestComputeRateDriftSingle(t *testing.T) {
	now := time.Now().UTC()
	snaps := []store.RateSnapshot{
		{RateDate: "2026-05-12", RoomTypeID: "STD", RatePlanID: "BAR", Amount: 199.0, SnapshotAt: now.Add(-72 * time.Hour)},
		{RateDate: "2026-05-12", RoomTypeID: "STD", RatePlanID: "BAR", Amount: 219.0, SnapshotAt: now.Add(-24 * time.Hour)},
	}
	got := computeRateDrift(snaps)
	if len(got) != 1 {
		t.Fatalf("expected 1 drift row, got %d", len(got))
	}
	if got[0].DeltaAmount != 20 {
		t.Errorf("expected delta 20, got %v", got[0].DeltaAmount)
	}
	if got[0].DeltaPct < 10.0 || got[0].DeltaPct > 10.1 {
		t.Errorf("expected delta_pct ~10.05, got %v", got[0].DeltaPct)
	}
}

func TestComputeRateDriftSkipsSingleCapture(t *testing.T) {
	snaps := []store.RateSnapshot{
		{RateDate: "2026-05-12", RoomTypeID: "STD", RatePlanID: "BAR", Amount: 199.0, SnapshotAt: time.Now()},
	}
	got := computeRateDrift(snaps)
	if len(got) != 0 {
		t.Errorf("expected zero rows when only one capture exists per group, got %v", got)
	}
}

func TestComputeOccupancyTrendDeltas(t *testing.T) {
	t1, _ := time.Parse("2006-01-02", "2026-05-01")
	t2, _ := time.Parse("2006-01-02", "2026-05-08")
	snaps := []store.DashboardSnapshot{
		{SnapshotDate: "2026-05-01", OccupancyPct: 70, ADR: 200, SnapshotAt: t1},
		{SnapshotDate: "2026-05-08", OccupancyPct: 80, ADR: 220, SnapshotAt: t2},
	}
	out := computeOccupancyTrend(snaps)
	if len(out) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(out))
	}
	if out[0].WoWOccPct != nil {
		t.Error("first row should not have WoW delta")
	}
	if out[1].WoWOccPct == nil || *out[1].WoWOccPct != 10 {
		t.Errorf("expected WoW occ delta 10, got %v", out[1].WoWOccPct)
	}
}

func TestStrIntFloatHelpers(t *testing.T) {
	if str(nil) != "" {
		t.Error("str(nil) should be empty")
	}
	if str("hello") != "hello" {
		t.Errorf("str(string) = %q", str("hello"))
	}
	if toInt(float64(42)) != 42 {
		t.Errorf("toInt(42.0) wrong")
	}
	if toFloat("3.14") != 3.14 {
		t.Errorf("toFloat(string) wrong")
	}
	if !truthy(true) || !truthy(1) || !truthy("true") {
		t.Error("truthy missed real truth")
	}
	if truthy(false) || truthy(0) || truthy("no") {
		t.Error("truthy admitted falsy")
	}
}
