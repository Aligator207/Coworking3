package repo

import (
	"context"
	"database/sql"
	"time"
)

type ReportRepo struct {
	DB *sql.DB
}

func NewReportRepo(db *sql.DB) *ReportRepo { return &ReportRepo{DB: db} }

// ReportStats aggregates booking activity for a period.
type ReportStats struct {
	From            time.Time
	To              time.Time
	Total           int
	Confirmed       int
	Completed       int
	CancelledByUser int
	CancelledByAdmin int
	PopularPlaces   []PlaceStat
	DailyLoad       []DayStat
	UserActivity    []UserStat
}

type PlaceStat struct {
	Name  string
	Type  string
	Count int
}

type DayStat struct {
	Day   string // YYYY-MM-DD
	Count int
}

type UserStat struct {
	Email string
	Name  string
	Count int
}

// Build computes aggregates for [from, to). Bookings are filtered by start_time
// to count what was scheduled in the period (independent of cancellation).
func (r *ReportRepo) Build(ctx context.Context, from, to time.Time) (*ReportStats, error) {
	stats := &ReportStats{From: from, To: to}

	// totals by status
	rows, err := r.DB.QueryContext(ctx, `
        SELECT status, COUNT(*)
          FROM bookings
         WHERE start_time >= $1 AND start_time < $2
         GROUP BY status`, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			rows.Close()
			return nil, err
		}
		stats.Total += n
		switch s {
		case "CONFIRMED":
			stats.Confirmed = n
		case "COMPLETED":
			stats.Completed = n
		case "CANCELLED_BY_USER":
			stats.CancelledByUser = n
		case "CANCELLED_BY_ADMIN":
			stats.CancelledByAdmin = n
		}
	}
	rows.Close()

	// popular places (top 5)
	rows, err = r.DB.QueryContext(ctx, `
        SELECT w.name, w.type, COUNT(*) AS c
          FROM bookings b
          JOIN workspaces w ON w.workspace_id = b.workspace_id
         WHERE b.start_time >= $1 AND b.start_time < $2
         GROUP BY w.name, w.type
         ORDER BY c DESC, w.name ASC
         LIMIT 5`, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p PlaceStat
		if err := rows.Scan(&p.Name, &p.Type, &p.Count); err != nil {
			rows.Close()
			return nil, err
		}
		stats.PopularPlaces = append(stats.PopularPlaces, p)
	}
	rows.Close()

	// daily load
	rows, err = r.DB.QueryContext(ctx, `
        SELECT to_char(start_time, 'YYYY-MM-DD') AS day, COUNT(*)
          FROM bookings
         WHERE start_time >= $1 AND start_time < $2
         GROUP BY day
         ORDER BY day ASC`, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d DayStat
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			rows.Close()
			return nil, err
		}
		stats.DailyLoad = append(stats.DailyLoad, d)
	}
	rows.Close()

	// user activity (top 10)
	rows, err = r.DB.QueryContext(ctx, `
        SELECT u.email, u.full_name, COUNT(*) AS c
          FROM bookings b
          JOIN users u ON u.user_id = b.user_id
         WHERE b.start_time >= $1 AND b.start_time < $2
         GROUP BY u.email, u.full_name
         ORDER BY c DESC, u.email ASC
         LIMIT 10`, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var u UserStat
		if err := rows.Scan(&u.Email, &u.Name, &u.Count); err != nil {
			rows.Close()
			return nil, err
		}
		stats.UserActivity = append(stats.UserActivity, u)
	}
	rows.Close()

	return stats, nil
}

// Save records the report metadata for audit. `data` is a JSON-serializable
// summary of the report.
func (r *ReportRepo) Save(ctx context.Context, from, to time.Time, dataJSON string, createdBy string) error {
	_, err := r.DB.ExecContext(ctx, `
        INSERT INTO reports (date_range_start, date_range_end, data, created_by)
        VALUES ($1, $2, $3::jsonb, $4)`, from, to, dataJSON, createdBy)
	return err
}

// SavedReport is a row from the reports table joined with the author's
// email/name. It's used to show the list of saved reports in the admin UI.
type SavedReport struct {
	ID          string
	GeneratedAt time.Time
	From        time.Time
	To          time.Time
	AuthorEmail string
	AuthorName  string
}

// ListRecent returns the most recent saved reports, newest first.
func (r *ReportRepo) ListRecent(ctx context.Context, limit int) ([]SavedReport, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.DB.QueryContext(ctx, `
        SELECT r.report_id,
               r.generated_at,
               r.date_range_start,
               r.date_range_end,
               COALESCE(u.email, ''),
               COALESCE(u.full_name, '')
          FROM reports r
          LEFT JOIN users u ON u.user_id = r.created_by
         ORDER BY r.generated_at DESC
         LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SavedReport, 0, limit)
	for rows.Next() {
		var s SavedReport
		if err := rows.Scan(&s.ID, &s.GeneratedAt, &s.From, &s.To, &s.AuthorEmail, &s.AuthorName); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// FindByID returns the raw JSON payload of a previously saved report.
// The empty string is returned with sql.ErrNoRows if no such report exists.
func (r *ReportRepo) FindByID(ctx context.Context, id string) (SavedReport, string, error) {
	var s SavedReport
	var data string
	err := r.DB.QueryRowContext(ctx, `
        SELECT r.report_id,
               r.generated_at,
               r.date_range_start,
               r.date_range_end,
               COALESCE(u.email, ''),
               COALESCE(u.full_name, ''),
               r.data::text
          FROM reports r
          LEFT JOIN users u ON u.user_id = r.created_by
         WHERE r.report_id = $1`, id).Scan(
		&s.ID, &s.GeneratedAt, &s.From, &s.To, &s.AuthorEmail, &s.AuthorName, &data)
	return s, data, err
}
