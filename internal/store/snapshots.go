// Snapshots: append-only history tables for time-series transcendence
// queries (housekeeping stale, rates drift, occupancy trend). The
// generator's per-endpoint tables UPSERT by primary key and lose
// history; these tables append a row per capture so we can compute
// time-in-status and rate/KPI deltas.
package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const snapshotSchema = `
CREATE TABLE IF NOT EXISTS housekeeping_snapshots (
    snapshot_id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    room_id TEXT NOT NULL,
    room_name TEXT,
    room_condition TEXT,
    housekeeper TEXT,
    housekeeper_id TEXT,
    room_occupied INTEGER,
    do_not_disturb INTEGER,
    frontdesk_status TEXT
);
CREATE INDEX IF NOT EXISTS idx_housekeeping_snapshots_room ON housekeeping_snapshots(room_id, snapshot_at);
CREATE INDEX IF NOT EXISTS idx_housekeeping_snapshots_at ON housekeeping_snapshots(snapshot_at);

CREATE TABLE IF NOT EXISTS rate_snapshots (
    snapshot_id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    rate_date DATE NOT NULL,
    room_type_id TEXT NOT NULL,
    rate_plan_id TEXT NOT NULL,
    amount REAL,
    currency TEXT
);
CREATE INDEX IF NOT EXISTS idx_rate_snapshots_lookup ON rate_snapshots(room_type_id, rate_plan_id, rate_date, snapshot_at);
CREATE INDEX IF NOT EXISTS idx_rate_snapshots_at ON rate_snapshots(snapshot_at);

CREATE TABLE IF NOT EXISTS dashboard_snapshots (
    snapshot_id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    snapshot_date DATE NOT NULL,
    total_rooms INTEGER,
    occupied_rooms INTEGER,
    occupancy_pct REAL,
    adr REAL,
    revpar REAL,
    revenue REAL,
    arrivals INTEGER,
    departures INTEGER,
    in_house INTEGER
);
CREATE INDEX IF NOT EXISTS idx_dashboard_snapshots_date ON dashboard_snapshots(snapshot_date, snapshot_at);
`

// EnsureSnapshotTables creates the snapshot tables if they don't exist.
// Safe to call repeatedly; CREATE TABLE IF NOT EXISTS is idempotent.
func (s *Store) EnsureSnapshotTables(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, stmt := range strings.Split(snapshotSchema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("snapshot migration: %w", err)
		}
	}
	return nil
}

// HousekeepingSnapshot is one captured row of room status.
type HousekeepingSnapshot struct {
	RoomID          string    `json:"room_id"`
	RoomName        string    `json:"room_name"`
	RoomCondition   string    `json:"room_condition"`
	Housekeeper     string    `json:"housekeeper"`
	HousekeeperID   string    `json:"housekeeper_id"`
	RoomOccupied    bool      `json:"room_occupied"`
	DoNotDisturb    bool      `json:"do_not_disturb"`
	FrontdeskStatus string    `json:"frontdesk_status"`
	SnapshotAt      time.Time `json:"snapshot_at"`
}

// SaveHousekeepingSnapshot appends one capture per room. snapshotAt
// defaults to CURRENT_TIMESTAMP when zero.
func (s *Store) SaveHousekeepingSnapshot(ctx context.Context, rooms []HousekeepingSnapshot) (int, error) {
	if err := s.EnsureSnapshotTables(ctx); err != nil {
		return 0, err
	}
	if len(rooms) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO housekeeping_snapshots
        (snapshot_at, room_id, room_name, room_condition, housekeeper, housekeeper_id, room_occupied, do_not_disturb, frontdesk_status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	count := 0
	for _, r := range rooms {
		snapAt := r.SnapshotAt
		if snapAt.IsZero() {
			snapAt = time.Now()
		}
		_, err := stmt.ExecContext(ctx, snapAt.UTC().Format(time.RFC3339), r.RoomID, r.RoomName, r.RoomCondition, r.Housekeeper, r.HousekeeperID, boolToInt(r.RoomOccupied), boolToInt(r.DoNotDisturb), r.FrontdeskStatus)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, tx.Commit()
}

// LoadHousekeepingSnapshots returns rows captured since `since`.
// roomID may be empty to return all rooms.
func (s *Store) LoadHousekeepingSnapshots(ctx context.Context, since time.Time, roomID string) ([]HousekeepingSnapshot, error) {
	if err := s.EnsureSnapshotTables(ctx); err != nil {
		return nil, err
	}
	q := `SELECT room_id, COALESCE(room_name,''), COALESCE(room_condition,''),
                 COALESCE(housekeeper,''), COALESCE(housekeeper_id,''),
                 COALESCE(room_occupied,0), COALESCE(do_not_disturb,0),
                 COALESCE(frontdesk_status,''), snapshot_at
          FROM housekeeping_snapshots WHERE snapshot_at >= ?`
	args := []any{since.UTC().Format(time.RFC3339)}
	if roomID != "" {
		q += ` AND room_id = ?`
		args = append(args, roomID)
	}
	q += ` ORDER BY snapshot_at ASC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HousekeepingSnapshot
	for rows.Next() {
		var r HousekeepingSnapshot
		var occ, dnd int
		var snap string
		if err := rows.Scan(&r.RoomID, &r.RoomName, &r.RoomCondition, &r.Housekeeper, &r.HousekeeperID, &occ, &dnd, &r.FrontdeskStatus, &snap); err != nil {
			return nil, err
		}
		r.RoomOccupied = occ != 0
		r.DoNotDisturb = dnd != 0
		r.SnapshotAt, _ = time.Parse(time.RFC3339, snap)
		out = append(out, r)
	}
	return out, rows.Err()
}

// RateSnapshot is one captured rate for (room_type, rate_plan, date).
type RateSnapshot struct {
	RateDate   string    `json:"rate_date"`
	RoomTypeID string    `json:"room_type_id"`
	RatePlanID string    `json:"rate_plan_id"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	SnapshotAt time.Time `json:"snapshot_at"`
}

func (s *Store) SaveRateSnapshot(ctx context.Context, rates []RateSnapshot) (int, error) {
	if err := s.EnsureSnapshotTables(ctx); err != nil {
		return 0, err
	}
	if len(rates) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO rate_snapshots
        (snapshot_at, rate_date, room_type_id, rate_plan_id, amount, currency)
        VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	count := 0
	for _, r := range rates {
		snapAt := r.SnapshotAt
		if snapAt.IsZero() {
			snapAt = time.Now()
		}
		_, err := stmt.ExecContext(ctx, snapAt.UTC().Format(time.RFC3339), r.RateDate, r.RoomTypeID, r.RatePlanID, r.Amount, r.Currency)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, tx.Commit()
}

func (s *Store) LoadRateSnapshots(ctx context.Context, since time.Time, roomTypeID string) ([]RateSnapshot, error) {
	if err := s.EnsureSnapshotTables(ctx); err != nil {
		return nil, err
	}
	q := `SELECT rate_date, room_type_id, rate_plan_id, COALESCE(amount,0),
                 COALESCE(currency,''), snapshot_at
          FROM rate_snapshots WHERE snapshot_at >= ?`
	args := []any{since.UTC().Format(time.RFC3339)}
	if roomTypeID != "" {
		q += ` AND room_type_id = ?`
		args = append(args, roomTypeID)
	}
	q += ` ORDER BY rate_date ASC, snapshot_at ASC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RateSnapshot
	for rows.Next() {
		var r RateSnapshot
		var snap string
		if err := rows.Scan(&r.RateDate, &r.RoomTypeID, &r.RatePlanID, &r.Amount, &r.Currency, &snap); err != nil {
			return nil, err
		}
		r.SnapshotAt, _ = time.Parse(time.RFC3339, snap)
		out = append(out, r)
	}
	return out, rows.Err()
}

// DashboardSnapshot captures occupancy/ADR/RevPAR for a date.
type DashboardSnapshot struct {
	SnapshotDate  string    `json:"snapshot_date"`
	TotalRooms    int       `json:"total_rooms"`
	OccupiedRooms int       `json:"occupied_rooms"`
	OccupancyPct  float64   `json:"occupancy_pct"`
	ADR           float64   `json:"adr"`
	RevPAR        float64   `json:"revpar"`
	Revenue       float64   `json:"revenue"`
	Arrivals      int       `json:"arrivals"`
	Departures    int       `json:"departures"`
	InHouse       int       `json:"in_house"`
	SnapshotAt    time.Time `json:"snapshot_at"`
}

func (s *Store) SaveDashboardSnapshot(ctx context.Context, d DashboardSnapshot) error {
	if err := s.EnsureSnapshotTables(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	snapAt := d.SnapshotAt
	if snapAt.IsZero() {
		snapAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO dashboard_snapshots
        (snapshot_at, snapshot_date, total_rooms, occupied_rooms, occupancy_pct, adr, revpar, revenue, arrivals, departures, in_house)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapAt.UTC().Format(time.RFC3339), d.SnapshotDate, d.TotalRooms, d.OccupiedRooms, d.OccupancyPct, d.ADR, d.RevPAR, d.Revenue, d.Arrivals, d.Departures, d.InHouse)
	return err
}

func (s *Store) LoadDashboardSnapshots(ctx context.Context, since time.Time) ([]DashboardSnapshot, error) {
	if err := s.EnsureSnapshotTables(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT snapshot_date,
        COALESCE(total_rooms,0), COALESCE(occupied_rooms,0),
        COALESCE(occupancy_pct,0), COALESCE(adr,0), COALESCE(revpar,0),
        COALESCE(revenue,0), COALESCE(arrivals,0), COALESCE(departures,0),
        COALESCE(in_house,0), snapshot_at
      FROM dashboard_snapshots WHERE snapshot_at >= ?
      ORDER BY snapshot_date ASC, snapshot_at ASC`, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DashboardSnapshot
	for rows.Next() {
		var d DashboardSnapshot
		var snap string
		if err := rows.Scan(&d.SnapshotDate, &d.TotalRooms, &d.OccupiedRooms, &d.OccupancyPct, &d.ADR, &d.RevPAR, &d.Revenue, &d.Arrivals, &d.Departures, &d.InHouse, &snap); err != nil {
			return nil, err
		}
		d.SnapshotAt, _ = time.Parse(time.RFC3339, snap)
		out = append(out, d)
	}
	return out, rows.Err()
}

// StaleRoom is a room that has remained dirty for at least the
// requested duration, computed from the snapshot history.
type StaleRoom struct {
	RoomID        string        `json:"room_id"`
	RoomName      string        `json:"room_name"`
	RoomCondition string        `json:"room_condition"`
	Housekeeper   string        `json:"housekeeper"`
	StatusSince   time.Time     `json:"status_since"`
	StaleFor      time.Duration `json:"stale_for_seconds"`
	StaleForHuman string        `json:"stale_for"`
}

// FindStaleRooms walks the housekeeping_snapshots history and returns
// rooms whose most-recent status is `condition` and which have been in
// that status for at least `minStale`. The "since" calculation is the
// timestamp of the earliest contiguous snapshot in the same status.
func (s *Store) FindStaleRooms(ctx context.Context, condition string, minStale time.Duration, lookback time.Duration) ([]StaleRoom, error) {
	if err := s.EnsureSnapshotTables(ctx); err != nil {
		return nil, err
	}
	since := time.Now().Add(-lookback)
	snaps, err := s.LoadHousekeepingSnapshots(ctx, since, "")
	if err != nil {
		return nil, err
	}
	// Group by room_id, walk from newest backward to find the first
	// snapshot where the condition flipped.
	byRoom := map[string][]HousekeepingSnapshot{}
	for _, snap := range snaps {
		byRoom[snap.RoomID] = append(byRoom[snap.RoomID], snap)
	}
	now := time.Now().UTC()
	var out []StaleRoom
	for _, rows := range byRoom {
		if len(rows) == 0 {
			continue
		}
		latest := rows[len(rows)-1]
		if !strings.EqualFold(latest.RoomCondition, condition) {
			continue
		}
		// Walk back to find first snapshot in same condition (contiguous)
		statusSince := latest.SnapshotAt
		for i := len(rows) - 1; i >= 0; i-- {
			if !strings.EqualFold(rows[i].RoomCondition, condition) {
				break
			}
			statusSince = rows[i].SnapshotAt
		}
		stale := now.Sub(statusSince)
		if stale < minStale {
			continue
		}
		out = append(out, StaleRoom{
			RoomID:        latest.RoomID,
			RoomName:      latest.RoomName,
			RoomCondition: latest.RoomCondition,
			Housekeeper:   latest.Housekeeper,
			StatusSince:   statusSince,
			StaleFor:      stale.Round(time.Minute),
			StaleForHuman: humanDur(stale),
		})
	}
	return out, nil
}

func humanDur(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
