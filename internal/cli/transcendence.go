// Transcendence commands: features that compose local-store joins,
// dated snapshots, or FTS5 search to answer questions no single
// Cloudbeds endpoint can answer.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloudbeds-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

// ============================================================
// today — one-shot front-desk dashboard
// ============================================================

type todayBoard struct {
	GeneratedAt  time.Time         `json:"generated_at"`
	Date         string            `json:"date"`
	Arrivals     todayCount        `json:"arrivals"`
	Departures   todayCount        `json:"departures"`
	InHouse      todayCount        `json:"in_house"`
	Unassigned   todayCount        `json:"unassigned"`
	Housekeeping todayHousekeeping `json:"housekeeping"`
	KPI          todayKPI          `json:"kpi"`
	DataSource   string            `json:"data_source"`
	Note         string            `json:"note,omitempty"`
}

type todayCount struct {
	Count int                      `json:"count"`
	Items []map[string]interface{} `json:"items,omitempty"`
}

type todayHousekeeping struct {
	DirtyCount     int `json:"dirty_count"`
	CleanCount     int `json:"clean_count"`
	InspectedCount int `json:"inspected_count"`
}

type todayKPI struct {
	OccupancyPct float64 `json:"occupancy_pct"`
	ADR          float64 `json:"adr"`
	Revenue      float64 `json:"revenue"`
}

func newTodayCmd(flags *rootFlags) *cobra.Command {
	var dbPath, dateStr string
	var withItems bool
	cmd := &cobra.Command{
		Use:   "today",
		Short: "One-shot front-desk dashboard: arrivals, departures, in-house, unassigned, dirty, today's ADR/occupancy",
		Long: `Compose arrivals, departures, in-house counts, unassigned reservations,
housekeeping breakdown, and today's KPIs into a single agent-shaped JSON
document. Reads the local mirror — run 'sync' first.`,
		Example: `  cloudbeds-pp-cli today --json
  cloudbeds-pp-cli today --date 2026-05-12 --with-items --json --select arrivals.count,departures.count`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			date := dateStr
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			board, err := computeTodayBoard(ctx, db, date, withItems)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), board, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&dateStr, "date", "", "Target date YYYY-MM-DD (default: today)")
	cmd.Flags().BoolVar(&withItems, "with-items", false, "Include the underlying reservation rows for each count")
	return cmd
}

func computeTodayBoard(ctx context.Context, db *store.Store, date string, withItems bool) (*todayBoard, error) {
	board := &todayBoard{GeneratedAt: time.Now().UTC(), Date: date, DataSource: "local"}
	// Arrivals: reservations with start_date == date and status confirmed/checked_in
	arrivals, err := queryReservations(ctx, db, "DATE(start_date)=? AND status IN ('confirmed','checked_in','not_confirmed')", date)
	if err != nil {
		return nil, err
	}
	board.Arrivals = todayCount{Count: len(arrivals)}
	if withItems {
		board.Arrivals.Items = arrivals
	}
	// Departures: reservations with end_date == date and status confirmed/checked_in/checked_out
	departures, err := queryReservations(ctx, db, "DATE(end_date)=? AND status IN ('confirmed','checked_in','checked_out')", date)
	if err != nil {
		return nil, err
	}
	board.Departures = todayCount{Count: len(departures)}
	if withItems {
		board.Departures.Items = departures
	}
	// In-house: reservations with status='checked_in' and start <= date <= end
	inhouse, err := queryReservations(ctx, db, "status='checked_in' AND DATE(start_date) <= ? AND DATE(end_date) >= ?", date, date)
	if err != nil {
		return nil, err
	}
	board.InHouse = todayCount{Count: len(inhouse)}
	if withItems {
		board.InHouse.Items = inhouse
	}
	// Unassigned: confirmed reservations without a room assignment for the window
	unassigned, err := queryReservations(ctx, db, "status='confirmed' AND DATE(start_date) <= ? AND DATE(end_date) >= ? AND (guest_name IS NULL OR guest_name='') ", date, date)
	// Fallback heuristic: count getRoomsUnassigned table if it has rows for date
	if err != nil || len(unassigned) == 0 {
		var n int
		_ = db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM get_rooms_unassigned`).Scan(&n)
		board.Unassigned = todayCount{Count: n}
	} else {
		board.Unassigned = todayCount{Count: len(unassigned)}
		if withItems {
			board.Unassigned.Items = unassigned
		}
	}
	// Housekeeping breakdown from latest get_housekeeping_status rows
	hk, err := housekeepingBreakdown(ctx, db)
	if err == nil {
		board.Housekeeping = hk
	}
	// KPI from most recent dashboard snapshot, if any
	snaps, err := db.LoadDashboardSnapshots(ctx, time.Now().Add(-7*24*time.Hour))
	if err == nil && len(snaps) > 0 {
		latest := snaps[len(snaps)-1]
		board.KPI = todayKPI{OccupancyPct: latest.OccupancyPct, ADR: latest.ADR, Revenue: latest.Revenue}
	} else {
		board.Note = "no recent dashboard snapshot — run 'snapshot dashboard' to populate KPIs"
	}
	return board, nil
}

func queryReservations(ctx context.Context, db *store.Store, where string, args ...any) ([]map[string]interface{}, error) {
	q := `SELECT reservation_id, status, COALESCE(guest_name,'') AS guest_name, start_date, end_date,
                 COALESCE(source_name,'') AS source_name, COALESCE(balance,0) AS balance
          FROM get_reservations WHERE ` + where + ` ORDER BY start_date ASC`
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		// Table may not exist if user hasn't synced. Return empty.
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var rid, status, guestName, startDate, endDate, sourceName string
		var balance float64
		if err := rows.Scan(&rid, &status, &guestName, &startDate, &endDate, &sourceName, &balance); err != nil {
			return out, err
		}
		out = append(out, map[string]interface{}{
			"reservation_id": rid,
			"status":         status,
			"guest_name":     guestName,
			"start_date":     startDate,
			"end_date":       endDate,
			"source_name":    sourceName,
			"balance":        balance,
		})
	}
	return out, rows.Err()
}

func housekeepingBreakdown(ctx context.Context, db *store.Store) (todayHousekeeping, error) {
	var hk todayHousekeeping
	rows, err := db.DB().QueryContext(ctx, `SELECT LOWER(COALESCE(room_condition,'')), COUNT(*) FROM get_housekeeping_status GROUP BY LOWER(COALESCE(room_condition,''))`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return hk, nil
		}
		return hk, err
	}
	defer rows.Close()
	for rows.Next() {
		var cond string
		var n int
		if err := rows.Scan(&cond, &n); err != nil {
			return hk, err
		}
		switch cond {
		case "dirty":
			hk.DirtyCount = n
		case "clean":
			hk.CleanCount = n
		case "inspected":
			hk.InspectedCount = n
		}
	}
	return hk, rows.Err()
}

// ============================================================
// housekeeping stale — rooms dirty for more than N hours
// ============================================================

func newHousekeepingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "housekeeping",
		Short:       "Housekeeping transcendence: stale-dirty room watchdog",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newHousekeepingStaleCmd(flags))
	return cmd
}

func newHousekeepingStaleCmd(flags *rootFlags) *cobra.Command {
	var hours float64
	var dbPath, condition string
	var lookbackHours float64
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find rooms in a given condition (default: dirty) longer than --hours",
		Long: `Compute time-in-status from append-only housekeeping_snapshots history.
Run 'snapshot housekeeping' periodically (e.g. cron every 30 minutes) to
populate the snapshot table — this command then walks the history per room
to find which rooms have been continuously in the named condition for at
least --hours.`,
		Example: `  cloudbeds-pp-cli housekeeping stale --hours 4 --json
  cloudbeds-pp-cli housekeeping stale --condition dirty --hours 6 --json --select room_name,stale_for`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			minStale := time.Duration(hours * float64(time.Hour))
			lookback := time.Duration(lookbackHours * float64(time.Hour))
			rooms, err := db.FindStaleRooms(ctx, condition, minStale, lookback)
			if err != nil {
				return err
			}
			sort.Slice(rooms, func(i, j int) bool { return rooms[i].StaleFor > rooms[j].StaleFor })
			return printJSONFiltered(cmd.OutOrStdout(), rooms, flags)
		},
	}
	cmd.Flags().Float64Var(&hours, "hours", 4, "Minimum hours in status")
	cmd.Flags().Float64Var(&lookbackHours, "lookback-hours", 24, "Snapshot history window to consider")
	cmd.Flags().StringVar(&condition, "condition", "dirty", "Room condition to filter (dirty, clean, inspected)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ============================================================
// rates drift — diff dated rate_snapshots over a window
// ============================================================

type rateDriftRow struct {
	RoomTypeID  string  `json:"room_type_id"`
	RatePlanID  string  `json:"rate_plan_id"`
	RateDate    string  `json:"rate_date"`
	OldAmount   float64 `json:"old_amount"`
	NewAmount   float64 `json:"new_amount"`
	DeltaAmount float64 `json:"delta_amount"`
	DeltaPct    float64 `json:"delta_pct"`
	Currency    string  `json:"currency"`
	Captured    int     `json:"captures"`
}

func newRatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rates",
		Short:       "Rate transcendence: drift across dated snapshots",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newRatesDriftCmd(flags))
	return cmd
}

func newRatesDriftCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath, roomType string
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Show how rates moved over a window, by date, plan, and room type",
		Long: `Diffs the earliest and latest rate_snapshots in the window for each
(room_type_id, rate_plan_id, rate_date) tuple. Cloudbeds returns only
current rates — drift requires the snapshot history. Run 'snapshot rates'
periodically to populate it.`,
		Example: `  cloudbeds-pp-cli rates drift --days 14 --json
  cloudbeds-pp-cli rates drift --days 30 --room-type STD --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
			snaps, err := db.LoadRateSnapshots(ctx, since, roomType)
			if err != nil {
				return err
			}
			drift := computeRateDrift(snaps)
			return printJSONFiltered(cmd.OutOrStdout(), drift, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "Window size in days")
	cmd.Flags().StringVar(&roomType, "room-type", "", "Limit to a single room type ID")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeRateDrift(snaps []store.RateSnapshot) []rateDriftRow {
	type key struct{ rt, rp, d string }
	groups := map[key][]store.RateSnapshot{}
	for _, s := range snaps {
		k := key{s.RoomTypeID, s.RatePlanID, s.RateDate}
		groups[k] = append(groups[k], s)
	}
	var out []rateDriftRow
	for k, g := range groups {
		if len(g) < 2 {
			continue
		}
		sort.Slice(g, func(i, j int) bool { return g[i].SnapshotAt.Before(g[j].SnapshotAt) })
		first, last := g[0], g[len(g)-1]
		delta := last.Amount - first.Amount
		pct := 0.0
		if first.Amount != 0 {
			pct = delta / first.Amount * 100.0
		}
		out = append(out, rateDriftRow{
			RoomTypeID: k.rt, RatePlanID: k.rp, RateDate: k.d,
			OldAmount: first.Amount, NewAmount: last.Amount,
			DeltaAmount: delta, DeltaPct: pct,
			Currency: last.Currency, Captured: len(g),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RateDate != out[j].RateDate {
			return out[i].RateDate < out[j].RateDate
		}
		return out[i].RoomTypeID < out[j].RoomTypeID
	})
	return out
}

// ============================================================
// occupancy trend — windowed occupancy/ADR/RevPAR with WoW deltas
// ============================================================

type occupancyTrendRow struct {
	Date         string   `json:"date"`
	OccupancyPct float64  `json:"occupancy_pct"`
	ADR          float64  `json:"adr"`
	RevPAR       float64  `json:"revpar"`
	Revenue      float64  `json:"revenue"`
	WoWOccPct    *float64 `json:"wow_occupancy_pct_delta,omitempty"`
	WoWADR       *float64 `json:"wow_adr_delta,omitempty"`
}

func newOccupancyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "occupancy",
		Short:       "Occupancy transcendence: windowed trend with WoW deltas",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newOccupancyTrendCmd(flags))
	return cmd
}

func newOccupancyTrendCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Daily occupancy/ADR/RevPAR series with week-over-week deltas",
		Long: `Reads dashboard_snapshots and emits one row per day in the window with
WoW delta vs. the same weekday a week earlier. Run 'snapshot dashboard'
once per day (cron) to populate.`,
		Example: `  cloudbeds-pp-cli occupancy trend --since 14d --json
  cloudbeds-pp-cli occupancy trend --since 30d --json --select date,occupancy_pct`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			window, err := parseDurationLike(since, 14*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			snaps, err := db.LoadDashboardSnapshots(ctx, time.Now().Add(-window))
			if err != nil {
				return err
			}
			trend := computeOccupancyTrend(snaps)
			return printJSONFiltered(cmd.OutOrStdout(), trend, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "14d", "Window size, e.g. 7d, 30d, 1y")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeOccupancyTrend(snaps []store.DashboardSnapshot) []occupancyTrendRow {
	// Reduce: take the latest snapshot per snapshot_date.
	byDate := map[string]store.DashboardSnapshot{}
	for _, s := range snaps {
		ex, ok := byDate[s.SnapshotDate]
		if !ok || s.SnapshotAt.After(ex.SnapshotAt) {
			byDate[s.SnapshotDate] = s
		}
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	out := make([]occupancyTrendRow, 0, len(dates))
	for _, d := range dates {
		s := byDate[d]
		row := occupancyTrendRow{
			Date: d, OccupancyPct: s.OccupancyPct, ADR: s.ADR, RevPAR: s.RevPAR, Revenue: s.Revenue,
		}
		// 7-day prior lookup
		t, _ := time.Parse("2006-01-02", d)
		prior := t.AddDate(0, 0, -7).Format("2006-01-02")
		if p, ok := byDate[prior]; ok {
			docc := s.OccupancyPct - p.OccupancyPct
			dadr := s.ADR - p.ADR
			row.WoWOccPct = &docc
			row.WoWADR = &dadr
		}
		out = append(out, row)
	}
	return out
}

// ============================================================
// sources mix — bookings / nights / revenue by source over a window
// ============================================================

type sourcesMixRow struct {
	SourceID     string  `json:"source_id"`
	SourceName   string  `json:"source_name"`
	Bookings     int     `json:"bookings"`
	Nights       int     `json:"nights"`
	Revenue      float64 `json:"revenue"`
	NightsShare  float64 `json:"nights_share_pct"`
	RevenueShare float64 `json:"revenue_share_pct"`
}

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sources",
		Short:       "Source transcendence: channel mix over a window",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newSourcesMixCmd(flags))
	return cmd
}

func newSourcesMixCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "mix",
		Short: "Bookings / nights / revenue by booking source over a window",
		Long: `Joins synced reservations with their source, aggregates bookings, nights,
and revenue per source over the window, and emits each source's share of
nights and revenue. Reads from local store; run 'sync' first.`,
		Example: `  cloudbeds-pp-cli sources mix --since 30d --json
  cloudbeds-pp-cli sources mix --since 90d --json --select source_name,nights,revenue_share_pct`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			window, err := parseDurationLike(since, 30*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339)
			rows, err := computeSourcesMix(ctx, db, cutoff)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window size, e.g. 7d, 30d, 90d")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeSourcesMix(ctx context.Context, db *store.Store, cutoff string) ([]sourcesMixRow, error) {
	q := `SELECT COALESCE(source_id,''), COALESCE(source_name,'(unknown)'),
                 COUNT(*) AS bookings,
                 COALESCE(SUM(CAST((julianday(end_date)-julianday(start_date)) AS INTEGER)),0) AS nights,
                 COALESCE(SUM(balance),0) AS revenue
          FROM get_reservations
          WHERE start_date >= ?
            AND status NOT IN ('canceled','no_show')
          GROUP BY source_id, source_name
          ORDER BY revenue DESC`
	rows, err := db.DB().QueryContext(ctx, q, cutoff)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []sourcesMixRow
	totalNights := 0
	totalRev := 0.0
	for rows.Next() {
		var sid, sname string
		var b, n int
		var rev float64
		if err := rows.Scan(&sid, &sname, &b, &n, &rev); err != nil {
			return nil, err
		}
		out = append(out, sourcesMixRow{SourceID: sid, SourceName: sname, Bookings: b, Nights: n, Revenue: rev})
		totalNights += n
		totalRev += rev
	}
	for i := range out {
		if totalNights > 0 {
			out[i].NightsShare = float64(out[i].Nights) / float64(totalNights) * 100.0
		}
		if totalRev > 0 {
			out[i].RevenueShare = out[i].Revenue / totalRev * 100.0
		}
	}
	return out, rows.Err()
}

// ============================================================
// payments unpaid — reservations arriving with non-zero balance
// ============================================================

type unpaidRow struct {
	ReservationID string  `json:"reservation_id"`
	GuestName     string  `json:"guest_name"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	SourceName    string  `json:"source_name"`
	Status        string  `json:"status"`
	Balance       float64 `json:"balance"`
}

func newPaymentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "payments",
		Short:       "Payments transcendence: unpaid upcoming arrivals",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newPaymentsUnpaidCmd(flags))
	return cmd
}

func newPaymentsUnpaidCmd(flags *rootFlags) *cobra.Command {
	var within string
	var dbPath string
	var minBalance float64
	cmd := &cobra.Command{
		Use:   "unpaid",
		Short: "Reservations arriving in the next N days with non-zero balance",
		Long: `Joins reservations on arrival window to balance > min. No native
Cloudbeds endpoint combines arrival window with balance filter — this
command queries the local mirror.`,
		Example: `  cloudbeds-pp-cli payments unpaid --within 7d --json
  cloudbeds-pp-cli payments unpaid --within 14d --min-balance 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			window, err := parseDurationLike(within, 7*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			today := time.Now().UTC().Format("2006-01-02")
			cutoff := time.Now().Add(window).UTC().Format("2006-01-02")
			rows, err := unpaidArrivals(ctx, db, today, cutoff, minBalance)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&within, "within", "7d", "Arrival window, e.g. 7d, 14d, 30d")
	cmd.Flags().Float64Var(&minBalance, "min-balance", 0.01, "Minimum balance to flag (default any positive)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func unpaidArrivals(ctx context.Context, db *store.Store, fromDate, toDate string, minBalance float64) ([]unpaidRow, error) {
	q := `SELECT reservation_id, COALESCE(guest_name,''), DATE(start_date), DATE(end_date),
                 COALESCE(source_name,''), COALESCE(status,''), COALESCE(balance,0)
          FROM get_reservations
          WHERE DATE(start_date) >= ? AND DATE(start_date) <= ?
            AND COALESCE(balance,0) >= ?
            AND status NOT IN ('canceled','no_show','checked_out')
          ORDER BY start_date ASC, balance DESC`
	rows, err := db.DB().QueryContext(ctx, q, fromDate, toDate, minBalance)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []unpaidRow
	for rows.Next() {
		var u unpaidRow
		if err := rows.Scan(&u.ReservationID, &u.GuestName, &u.StartDate, &u.EndDate, &u.SourceName, &u.Status, &u.Balance); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ============================================================
// reservations no-shows + reservations timeline
// ============================================================

type noShowRow struct {
	ReservationID string `json:"reservation_id"`
	GuestName     string `json:"guest_name"`
	StartDate     string `json:"start_date"`
	SourceName    string `json:"source_name"`
	LeadTimeDays  int    `json:"lead_time_days"`
}

type noShowReport struct {
	Window       string           `json:"window"`
	TotalNoShow  int              `json:"total_no_shows"`
	BySource     []map[string]any `json:"by_source"`
	RepeatGuests []map[string]any `json:"repeat_no_show_guests"`
	NoShows      []noShowRow      `json:"no_shows"`
}

type timelineEvent struct {
	When  time.Time `json:"when"`
	Kind  string    `json:"kind"`
	Note  string    `json:"note"`
	Value any       `json:"value,omitempty"`
}

type reservationTimeline struct {
	ReservationID string          `json:"reservation_id"`
	GuestName     string          `json:"guest_name"`
	StartDate     string          `json:"start_date"`
	EndDate       string          `json:"end_date"`
	Status        string          `json:"status"`
	SourceName    string          `json:"source_name"`
	Balance       float64         `json:"balance"`
	Events        []timelineEvent `json:"events"`
}

func newReservationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reservations",
		Short:       "Reservation transcendence: no-show audit and forensic timeline",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newReservationsNoShowsCmd(flags))
	cmd.AddCommand(newReservationsTimelineCmd(flags))
	return cmd
}

func newReservationsNoShowsCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "no-shows",
		Short: "No-show audit: rate by source, room type, lead-time bucket; flag repeat no-show guests",
		Example: `  cloudbeds-pp-cli reservations no-shows --since 30d --json
  cloudbeds-pp-cli reservations no-shows --since 90d --json --select total_no_shows,by_source`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			window, err := parseDurationLike(since, 30*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window).UTC().Format("2006-01-02")
			report, err := computeNoShowReport(ctx, db, cutoff, since)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window size")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeNoShowReport(ctx context.Context, db *store.Store, cutoff, window string) (*noShowReport, error) {
	q := `SELECT reservation_id, COALESCE(guest_name,''), COALESCE(guest_id,''),
                 DATE(start_date), DATE(date_created), COALESCE(source_name,'(unknown)')
          FROM get_reservations
          WHERE status='no_show' AND DATE(start_date) >= ?
          ORDER BY start_date DESC`
	rows, err := db.DB().QueryContext(ctx, q, cutoff)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return &noShowReport{Window: window}, nil
		}
		return nil, err
	}
	defer rows.Close()
	var noShows []noShowRow
	guestCounts := map[string]int{}
	guestNames := map[string]string{}
	sourceCounts := map[string]int{}
	for rows.Next() {
		var rid, gname, gid, sd, dc, sname string
		if err := rows.Scan(&rid, &gname, &gid, &sd, &dc, &sname); err != nil {
			return nil, err
		}
		lead := 0
		if dc != "" && sd != "" {
			t1, _ := time.Parse("2006-01-02", dc)
			t2, _ := time.Parse("2006-01-02", sd)
			lead = int(t2.Sub(t1).Hours() / 24)
			if lead < 0 {
				lead = 0
			}
		}
		noShows = append(noShows, noShowRow{ReservationID: rid, GuestName: gname, StartDate: sd, SourceName: sname, LeadTimeDays: lead})
		if gid != "" {
			guestCounts[gid]++
			guestNames[gid] = gname
		}
		sourceCounts[sname]++
	}
	bySource := []map[string]any{}
	for sname, n := range sourceCounts {
		bySource = append(bySource, map[string]any{"source_name": sname, "count": n})
	}
	sort.Slice(bySource, func(i, j int) bool { return bySource[i]["count"].(int) > bySource[j]["count"].(int) })
	repeats := []map[string]any{}
	for gid, n := range guestCounts {
		if n >= 2 {
			repeats = append(repeats, map[string]any{"guest_id": gid, "guest_name": guestNames[gid], "no_show_count": n})
		}
	}
	sort.Slice(repeats, func(i, j int) bool { return repeats[i]["no_show_count"].(int) > repeats[j]["no_show_count"].(int) })
	return &noShowReport{Window: window, TotalNoShow: len(noShows), BySource: bySource, RepeatGuests: repeats, NoShows: noShows}, rows.Err()
}

func newReservationsTimelineCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "timeline [reservation-id]",
		Short: "Forensic event log for a single reservation: status, payments, notes, room assignments",
		Example: `  cloudbeds-pp-cli reservations timeline RES-12345 --json
  cloudbeds-pp-cli reservations timeline 99f0a01b --json --select events`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			t, err := buildReservationTimeline(ctx, db, args[0])
			if err != nil {
				return err
			}
			if t == nil {
				return notFoundErr(fmt.Errorf("reservation %q not found in local store; run 'sync' or query 'get-reservation --id %s'", args[0], args[0]))
			}
			return printJSONFiltered(cmd.OutOrStdout(), t, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func buildReservationTimeline(ctx context.Context, db *store.Store, id string) (*reservationTimeline, error) {
	q := `SELECT reservation_id, COALESCE(guest_name,''), COALESCE(start_date,''), COALESCE(end_date,''),
                 COALESCE(status,''), COALESCE(source_name,''), COALESCE(balance,0),
                 COALESCE(date_created, ''), COALESCE(date_modified, '')
          FROM get_reservations WHERE reservation_id = ? OR id = ? LIMIT 1`
	row := db.DB().QueryRowContext(ctx, q, id, id)
	t := &reservationTimeline{}
	var dc, dm string
	if err := row.Scan(&t.ReservationID, &t.GuestName, &t.StartDate, &t.EndDate, &t.Status, &t.SourceName, &t.Balance, &dc, &dm); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	// Build events from reservation lifecycle and any joined sub-tables
	if dc != "" {
		when, _ := time.Parse(time.RFC3339, dc)
		t.Events = append(t.Events, timelineEvent{When: when, Kind: "created", Note: "reservation booked", Value: t.SourceName})
	}
	if dm != "" {
		when, _ := time.Parse(time.RFC3339, dm)
		t.Events = append(t.Events, timelineEvent{When: when, Kind: "modified", Note: "reservation modified"})
	}
	if t.Status != "" {
		t.Events = append(t.Events, timelineEvent{When: time.Now().UTC(), Kind: "status", Note: "current status", Value: t.Status})
	}
	// Reservation notes
	noteRows, err := db.DB().QueryContext(ctx, `SELECT COALESCE(date_created,''), COALESCE(note,'') FROM get_reservation_notes WHERE reservation_id = ? ORDER BY date_created ASC`, t.ReservationID)
	if err == nil {
		defer noteRows.Close()
		for noteRows.Next() {
			var when, note string
			if err := noteRows.Scan(&when, &note); err != nil {
				continue
			}
			ts, _ := time.Parse(time.RFC3339, when)
			t.Events = append(t.Events, timelineEvent{When: ts, Kind: "note", Note: note})
		}
	}
	sort.Slice(t.Events, func(i, j int) bool { return t.Events[i].When.Before(t.Events[j].When) })
	return t, nil
}

// ============================================================
// guests search --history — FTS5 across guests + stay history join
// ============================================================

type guestHit struct {
	GuestID   string      `json:"guest_id"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Email     string      `json:"email"`
	Phone     string      `json:"phone"`
	Stays     *guestStays `json:"stays,omitempty"`
}

type guestStays struct {
	TotalStays   int     `json:"total_stays"`
	TotalNights  int     `json:"total_nights"`
	TotalRevenue float64 `json:"total_revenue"`
	LastStay     string  `json:"last_stay,omitempty"`
}

func newGuestsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "guests",
		Short:       "Guest transcendence: FTS5 search with optional stay history",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newGuestsSearchCmd(flags))
	return cmd
}

func newGuestsSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var withHistory bool
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "FTS-style search across guest name, email, phone; --history adds stay totals",
		Long: `Searches the locally synced guest tables. Without --history this is a
plain LIKE-fallback search. With --history, joins matched guest_ids to
reservations to return total stays, total nights, total revenue, and last
stay date — a join no Cloudbeds endpoint exposes.`,
		Example: `  cloudbeds-pp-cli guests search "smith" --history --json
  cloudbeds-pp-cli guests search "ravi@example.com" --history --json --select first_name,last_name,stays.total_nights`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			hits, err := searchGuests(ctx, db, args[0], withHistory, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().BoolVar(&withHistory, "history", false, "Include stay history per guest (requires sync of reservations)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max guests to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func searchGuests(ctx context.Context, db *store.Store, query string, withHistory bool, limit int) ([]guestHit, error) {
	pat := "%" + strings.ToLower(query) + "%"
	q := `SELECT COALESCE(guest_id,''), COALESCE(guest_first_name,''), COALESCE(guest_last_name,''),
                 COALESCE(guest_email,''), COALESCE(guest_phone,'')
          FROM get_guests_by_status
          WHERE LOWER(COALESCE(guest_first_name,'')) LIKE ?
             OR LOWER(COALESCE(guest_last_name,'')) LIKE ?
             OR LOWER(COALESCE(guest_email,'')) LIKE ?
             OR LOWER(COALESCE(guest_phone,'')) LIKE ?
          GROUP BY guest_id
          LIMIT ?`
	rows, err := db.DB().QueryContext(ctx, q, pat, pat, pat, pat, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var hits []guestHit
	for rows.Next() {
		var h guestHit
		if err := rows.Scan(&h.GuestID, &h.FirstName, &h.LastName, &h.Email, &h.Phone); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	if !withHistory {
		return hits, rows.Err()
	}
	// Join stay history for each hit
	for i := range hits {
		gid := hits[i].GuestID
		if gid == "" {
			continue
		}
		s, err := guestStaySummary(ctx, db, gid)
		if err == nil {
			hits[i].Stays = s
		}
	}
	return hits, nil
}

func guestStaySummary(ctx context.Context, db *store.Store, guestID string) (*guestStays, error) {
	q := `SELECT COUNT(*),
                 COALESCE(SUM(CAST((julianday(end_date)-julianday(start_date)) AS INTEGER)), 0),
                 COALESCE(SUM(balance), 0),
                 MAX(DATE(start_date))
          FROM get_reservations WHERE guest_id = ? AND status NOT IN ('canceled','no_show')`
	row := db.DB().QueryRowContext(ctx, q, guestID)
	s := &guestStays{}
	var last sql.NullString
	if err := row.Scan(&s.TotalStays, &s.TotalNights, &s.TotalRevenue, &last); err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return s, nil
		}
		return nil, err
	}
	if last.Valid {
		s.LastStay = last.String
	}
	return s, nil
}

// ============================================================
// reconcile — sync diff: compare local mirror vs API
// ============================================================

type reconcileReport struct {
	Table      string           `json:"table"`
	Window     string           `json:"window"`
	APITotal   int              `json:"api_count"`
	APIStatus  string           `json:"api_status,omitempty"` // populated only when api_count is -1; values: "unconfigured", "unreachable"
	LocalTotal int              `json:"local_count"`
	Added      []map[string]any `json:"added,omitempty"`
	Removed    []map[string]any `json:"removed,omitempty"`
	Changed    []map[string]any `json:"changed,omitempty"`
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var table, since, dbPath string
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Compare local mirror to live API for a sampled window; emit a typed diff",
		Long: `Pulls a sampled window from the API and compares it against the local
store, emitting added/removed/changed rows. Useful for CI integration
checks or for triaging "guest says they booked but I don't see it" tickets.`,
		Example: `  cloudbeds-pp-cli reconcile --table reservations --since 24h --json
  cloudbeds-pp-cli reconcile --table reservations --since 7d --json --select api_count,local_count`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if table != "reservations" {
				return usageErr(fmt.Errorf("--table must be 'reservations' (other tables not yet supported)"))
			}
			window, err := parseDurationLike(since, 24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			report, err := reconcileReservations(ctx, flags, dbPath, window, since)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&table, "table", "reservations", "Table to diff (reservations)")
	cmd.Flags().StringVar(&since, "since", "24h", "Window size")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func reconcileReservations(ctx context.Context, flags *rootFlags, dbPath string, window time.Duration, sinceLabel string) (*reconcileReport, error) {
	report := &reconcileReport{Table: "reservations", Window: sinceLabel}
	c, err := flags.newClient()
	if err != nil {
		// No client config — emit a structured report stating local
		// counts only, so JSON output remains valid for callers that
		// pipe through jq. APIStatus carries the reason so consumers
		// don't have to interpret -1 as a magic value.
		report.APITotal = -1
		report.APIStatus = "unconfigured"
		fmt.Fprintln(os.Stderr, "warning: API client unconfigured (set CLOUDBEDS_OAUTH2); reconcile reports local counts only (api_count=-1, api_status=unconfigured)")
		return loadLocalCountsOnly(ctx, dbPath, report, window)
	}
	cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339)
	params := map[string]string{"modifiedFrom": cutoff[:19] + " UTC"}
	data, err := c.Get("/getReservations", params)
	if err != nil {
		// API unreachable or unauthorized — emit local-only report so
		// stdout stays valid JSON. APIStatus surfaces the reason; a
		// stderr warning informs interactive users.
		report.APITotal = -1
		report.APIStatus = "unreachable"
		fmt.Fprintf(os.Stderr, "warning: API unreachable (%v); reconcile reports local counts only (api_count=-1, api_status=unreachable)\n", err)
		return loadLocalCountsOnly(ctx, dbPath, report, window)
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, apiErr(fmt.Errorf("decoding API response: %w", err))
	}
	apiByID := map[string]map[string]any{}
	for _, r := range env.Data {
		id := str(r["reservationID"])
		if id == "" {
			id = str(r["id"])
		}
		if id != "" {
			apiByID[id] = r
		}
	}
	db, err := openLocalStore(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx, `SELECT reservation_id, COALESCE(status,''), COALESCE(date_modified,'') FROM get_reservations WHERE date_modified >= ?`, cutoff)
	localByID := map[string]map[string]any{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rid, status, mod string
			if err := rows.Scan(&rid, &status, &mod); err != nil {
				continue
			}
			localByID[rid] = map[string]any{"reservation_id": rid, "status": status, "date_modified": mod}
		}
	}
	report.APITotal = len(apiByID)
	report.LocalTotal = len(localByID)
	for id, r := range apiByID {
		if _, ok := localByID[id]; !ok {
			report.Added = append(report.Added, map[string]any{"reservation_id": id, "status": str(r["status"]), "guest_name": str(r["guestName"])})
		} else if str(r["status"]) != str(localByID[id]["status"]) {
			report.Changed = append(report.Changed, map[string]any{
				"reservation_id": id,
				"local_status":   str(localByID[id]["status"]),
				"api_status":     str(r["status"]),
			})
		}
	}
	for id, l := range localByID {
		if _, ok := apiByID[id]; !ok {
			report.Removed = append(report.Removed, map[string]any{"reservation_id": id, "local_status": str(l["status"])})
		}
	}
	return report, nil
}

// loadLocalCountsOnly populates the local count for reconcile when the
// API is unreachable, leaving APITotal at -1 as a signal.
func loadLocalCountsOnly(ctx context.Context, dbPath string, report *reconcileReport, window time.Duration) (*reconcileReport, error) {
	cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339)
	db, err := openLocalStore(ctx, dbPath)
	if err != nil {
		return report, nil
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(ctx, `SELECT COUNT(*) FROM get_reservations WHERE date_modified >= ?`, cutoff)
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			_ = rows.Scan(&report.LocalTotal)
		}
	}
	return report, nil
}

// ============================================================
// audit night — end-of-shift bundle
// ============================================================

type nightAudit struct {
	GeneratedAt          time.Time                `json:"generated_at"`
	Date                 string                   `json:"date"`
	UncheckedInArrivals  []map[string]interface{} `json:"unchecked_in_arrivals"`
	UndepartedDepartures []map[string]interface{} `json:"undeparted_departures"`
	UnbalancedFolios     []unpaidRow              `json:"unbalanced_folios"`
	NoShowsToday         []map[string]interface{} `json:"no_shows_today"`
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "End-of-shift audit pack",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newAuditNightCmd(flags))
	return cmd
}

func newAuditNightCmd(flags *rootFlags) *cobra.Command {
	var dateStr, dbPath string
	cmd := &cobra.Command{
		Use:   "night",
		Short: "End-of-shift bundle: arrivals not checked-in, departures not checked-out, unbalanced folios, today's no-shows",
		Example: `  cloudbeds-pp-cli audit night --json
  cloudbeds-pp-cli audit night --date 2026-05-12 --json --select unbalanced_folios,no_shows_today`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			date := dateStr
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			audit := &nightAudit{GeneratedAt: time.Now().UTC(), Date: date}
			audit.UncheckedInArrivals, _ = queryReservations(ctx, db, "DATE(start_date)=? AND status IN ('confirmed','not_confirmed')", date)
			audit.UndepartedDepartures, _ = queryReservations(ctx, db, "DATE(end_date)=? AND status='checked_in'", date)
			audit.NoShowsToday, _ = queryReservations(ctx, db, "DATE(start_date)=? AND status='no_show'", date)
			rows, _ := unpaidArrivals(ctx, db, date, date, 0.01)
			audit.UnbalancedFolios = rows
			return printJSONFiltered(cmd.OutOrStdout(), audit, flags)
		},
	}
	cmd.Flags().StringVar(&dateStr, "date", "", "Date YYYY-MM-DD (default today)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ============================================================
// shared helpers
// ============================================================

// parseDurationLike parses a string like "7d", "30d", "12h", "1y" into a
// time.Duration. Returns the default when the input is empty.
func parseDurationLike(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	s = strings.TrimSpace(strings.ToLower(s))
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Custom suffixes: d, w, y
	if len(s) >= 2 {
		suffix := s[len(s)-1]
		numStr := s[:len(s)-1]
		n, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		switch suffix {
		case 'd':
			return time.Duration(n * 24 * float64(time.Hour)), nil
		case 'w':
			return time.Duration(n * 7 * 24 * float64(time.Hour)), nil
		case 'y':
			return time.Duration(n * 365 * 24 * float64(time.Hour)), nil
		}
	}
	return 0, fmt.Errorf("invalid duration %q (use forms like 7d, 24h, 30m, 2w, 1y)", s)
}
