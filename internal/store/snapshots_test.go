package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmp := t.TempDir()
	s, err := OpenWithContext(context.Background(), filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSaveLoadHousekeepingSnapshot(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Add(-2 * time.Hour)
	rows := []HousekeepingSnapshot{
		{RoomID: "R1", RoomName: "Room 1", RoomCondition: "dirty", SnapshotAt: now},
		{RoomID: "R2", RoomName: "Room 2", RoomCondition: "clean", SnapshotAt: now.Add(-time.Minute)},
	}
	if n, err := s.SaveHousekeepingSnapshot(ctx, rows); err != nil || n != 2 {
		t.Fatalf("save: n=%d err=%v", n, err)
	}
	got, err := s.LoadHousekeepingSnapshots(ctx, time.Now().Add(-24*time.Hour), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
}

func TestFindStaleRooms(t *testing.T) {
	cases := []struct {
		name     string
		rows     []HousekeepingSnapshot
		minStale time.Duration
		want     int
	}{
		{
			name: "single dirty room over threshold",
			rows: []HousekeepingSnapshot{
				{RoomID: "R1", RoomCondition: "dirty", SnapshotAt: time.Now().Add(-3 * time.Hour)},
				{RoomID: "R1", RoomCondition: "dirty", SnapshotAt: time.Now().Add(-1 * time.Hour)},
			},
			minStale: 2 * time.Hour,
			want:     1,
		},
		{
			name: "dirty room recently changed under threshold",
			rows: []HousekeepingSnapshot{
				{RoomID: "R2", RoomCondition: "clean", SnapshotAt: time.Now().Add(-3 * time.Hour)},
				{RoomID: "R2", RoomCondition: "dirty", SnapshotAt: time.Now().Add(-30 * time.Minute)},
			},
			minStale: 2 * time.Hour,
			want:     0,
		},
		{
			name: "currently clean room with prior dirty does not count",
			rows: []HousekeepingSnapshot{
				{RoomID: "R3", RoomCondition: "dirty", SnapshotAt: time.Now().Add(-5 * time.Hour)},
				{RoomID: "R3", RoomCondition: "clean", SnapshotAt: time.Now().Add(-1 * time.Hour)},
			},
			minStale: 2 * time.Hour,
			want:     0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()
			if _, err := s.SaveHousekeepingSnapshot(ctx, tc.rows); err != nil {
				t.Fatal(err)
			}
			got, err := s.FindStaleRooms(ctx, "dirty", tc.minStale, 24*time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Errorf("expected %d stale rooms, got %d (%v)", tc.want, len(got), got)
			}
		})
	}
}

func TestSaveLoadRateSnapshot(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	rates := []RateSnapshot{
		{RateDate: "2026-05-12", RoomTypeID: "STD", RatePlanID: "BAR", Amount: 199.0, Currency: "USD"},
		{RateDate: "2026-05-13", RoomTypeID: "STD", RatePlanID: "BAR", Amount: 219.0, Currency: "USD"},
	}
	if n, err := s.SaveRateSnapshot(ctx, rates); err != nil || n != 2 {
		t.Fatalf("save: n=%d err=%v", n, err)
	}
	got, err := s.LoadRateSnapshots(ctx, time.Now().Add(-1*time.Hour), "STD")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
}

func TestSaveLoadDashboardSnapshot(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	d := DashboardSnapshot{SnapshotDate: "2026-05-10", TotalRooms: 60, OccupiedRooms: 48, OccupancyPct: 80.0, ADR: 245.0, RevPAR: 196.0, Revenue: 11760.0}
	if err := s.SaveDashboardSnapshot(ctx, d); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadDashboardSnapshots(ctx, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].OccupancyPct != 80.0 {
		t.Errorf("expected occupancy 80, got %v", got[0].OccupancyPct)
	}
}

func TestHumanDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
		{30 * time.Hour, "1d6h"},
	}
	for _, tc := range cases {
		if got := humanDur(tc.d); got != tc.want {
			t.Errorf("humanDur(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("true should be 1")
	}
	if boolToInt(false) != 0 {
		t.Error("false should be 0")
	}
}
