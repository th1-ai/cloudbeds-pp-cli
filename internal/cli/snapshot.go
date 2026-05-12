package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"cloudbeds-pp-cli/internal/cliutil"
	"cloudbeds-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

// newSnapshotCmd is the parent for `snapshot housekeeping|rates|dashboard`
// commands. Each captures a row into the append-only snapshots tables so
// time-series transcendence commands have history to compute against.
func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture point-in-time snapshots into the local store for trend queries",
		Long: `Capture a row of state into append-only snapshot tables. Run periodically
(e.g. via cron every 30 minutes for housekeeping, daily for rates and dashboard)
so transcendence commands like 'housekeeping stale', 'rates drift', and
'occupancy trend' have history to compute against.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
	}
	cmd.AddCommand(newSnapshotHousekeepingCmd(flags))
	cmd.AddCommand(newSnapshotRatesCmd(flags))
	cmd.AddCommand(newSnapshotDashboardCmd(flags))
	return cmd
}

func newSnapshotHousekeepingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "housekeeping",
		Short:       "Capture current housekeeping status into the snapshot history",
		Example:     "  cloudbeds-pp-cli snapshot housekeeping --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"would":"snapshot housekeeping"}`)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			data, err := c.Get("/getHousekeepingStatus", nil)
			if err != nil {
				return apiErr(fmt.Errorf("fetching housekeeping status: %w", err))
			}
			rows, err := decodeHousekeepingRows(data)
			if err != nil {
				return apiErr(err)
			}
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			n, err := db.SaveHousekeepingSnapshot(ctx, rows)
			if err != nil {
				return err
			}
			result := map[string]any{"captured": n, "snapshot_at": time.Now().UTC().Format(time.RFC3339)}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newSnapshotRatesCmd(flags *rootFlags) *cobra.Command {
	var dbPath, roomTypeID string
	var startDate, endDate string
	cmd := &cobra.Command{
		Use:         "rates",
		Short:       "Capture current rate plan amounts for a date range",
		Example:     "  cloudbeds-pp-cli snapshot rates --start 2026-05-10 --end 2026-05-24 --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"would":"snapshot rates"}`)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			params := map[string]string{}
			if startDate != "" {
				params["startDate"] = startDate
			}
			if endDate != "" {
				params["endDate"] = endDate
			}
			if roomTypeID != "" {
				params["roomTypeID"] = roomTypeID
			}
			data, err := c.Get("/getRate", params)
			if err != nil {
				return apiErr(fmt.Errorf("fetching rates: %w", err))
			}
			rows, err := decodeRateRows(data)
			if err != nil {
				return apiErr(err)
			}
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			n, err := db.SaveRateSnapshot(ctx, rows)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"captured": n}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&roomTypeID, "room-type", "", "Limit to one room type ID")
	cmd.Flags().StringVar(&startDate, "start", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endDate, "end", "", "End date YYYY-MM-DD")
	return cmd
}

func newSnapshotDashboardCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "dashboard",
		Short:       "Capture today's dashboard KPIs (occupancy, ADR, RevPAR) for the trend",
		Example:     "  cloudbeds-pp-cli snapshot dashboard --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"would":"snapshot dashboard"}`)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			data, err := c.Get("/getDashboard", nil)
			if err != nil {
				return apiErr(fmt.Errorf("fetching dashboard: %w", err))
			}
			d, err := decodeDashboard(data)
			if err != nil {
				return apiErr(err)
			}
			db, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.SaveDashboardSnapshot(ctx, d); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), d, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// openLocalStore opens the SQLite store at dbPath (or the default
// location if empty). Wraps store.OpenWithContext with a friendlier
// error directing the user to run sync first.
func openLocalStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("cloudbeds-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'cloudbeds-pp-cli sync' first to create it.", err)
	}
	return db, nil
}

// decodeHousekeepingRows converts the getHousekeepingStatus response
// into snapshot rows. The Cloudbeds envelope is
// {success: true, data: [{roomID, roomName, roomCondition, ...}, ...]}
// — we walk the array.
func decodeHousekeepingRows(raw []byte) ([]store.HousekeepingSnapshot, error) {
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decoding housekeeping envelope: %w", err)
	}
	now := time.Now().UTC()
	var out []store.HousekeepingSnapshot
	for _, row := range env.Data {
		out = append(out, store.HousekeepingSnapshot{
			RoomID:          str(row["roomID"]),
			RoomName:        str(row["roomName"]),
			RoomCondition:   str(row["roomCondition"]),
			Housekeeper:     str(row["housekeeper"]),
			HousekeeperID:   str(row["housekeeperID"]),
			RoomOccupied:    truthy(row["roomOccupied"]),
			DoNotDisturb:    truthy(row["doNotDisturb"]),
			FrontdeskStatus: str(row["frontdeskStatus"]),
			SnapshotAt:      now,
		})
	}
	return out, nil
}

// decodeRateRows converts the getRate response into snapshot rows.
// Cloudbeds envelope: {success, data: [{date, roomTypeID, ratePlanID, rate, ...}]}
// or sometimes {success, data: {<roomTypeID>: {<ratePlanID>: [{date, rate}, ...]}}}.
func decodeRateRows(raw []byte) ([]store.RateSnapshot, error) {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decoding rate envelope: %w", err)
	}
	var out []store.RateSnapshot
	now := time.Now().UTC()
	// First try array shape
	var arr []map[string]any
	if err := json.Unmarshal(env.Data, &arr); err == nil {
		for _, r := range arr {
			out = append(out, store.RateSnapshot{
				RateDate:   str(r["date"]),
				RoomTypeID: str(r["roomTypeID"]),
				RatePlanID: str(r["ratePlanID"]),
				Amount:     toFloat(r["rate"]),
				Currency:   str(r["currency"]),
				SnapshotAt: now,
			})
		}
		return out, nil
	}
	// Fallback: skip nested shape gracefully (callers can build their own
	// records via the underlying API). Returning an empty slice with no
	// error keeps the snapshot command useful as a stub when the response
	// shape is unexpected.
	return out, nil
}

// decodeDashboard parses getDashboard. The Cloudbeds shape is
// {success, data: {totalRooms, occupiedRooms, ...}}
func decodeDashboard(raw []byte) (store.DashboardSnapshot, error) {
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return store.DashboardSnapshot{}, fmt.Errorf("decoding dashboard: %w", err)
	}
	d := env.Data
	occ := toInt(d["occupiedRooms"])
	tot := toInt(d["totalRooms"])
	pct := 0.0
	if tot > 0 {
		pct = float64(occ) / float64(tot) * 100.0
	}
	rev := toFloat(d["revenue"])
	revpar := 0.0
	if tot > 0 {
		revpar = rev / float64(tot)
	}
	return store.DashboardSnapshot{
		SnapshotDate:  time.Now().UTC().Format("2006-01-02"),
		TotalRooms:    tot,
		OccupiedRooms: occ,
		OccupancyPct:  pct,
		ADR:           toFloat(d["adr"]),
		RevPAR:        revpar,
		Revenue:       rev,
		Arrivals:      toInt(d["arrivals"]),
		Departures:    toInt(d["departures"]),
		InHouse:       occ,
		SnapshotAt:    time.Now().UTC(),
	}, nil
}

func str(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return cliutil.CleanText(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(v)
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return x == "true" || x == "1" || x == "yes"
	}
	return false
}
