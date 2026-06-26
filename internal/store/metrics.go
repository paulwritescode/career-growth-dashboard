package store

import (
	"context"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// DayPublishStat summarizes published tiers for a single post date.
type DayPublishStat struct {
	Date        string
	Published   int // total tiers published that day
	Credibility int // published blog/linkedin tiers that day
}

// PublishStatsByDate returns per-date published-tier counts for posts whose
// post_date falls within [from, to] (inclusive, YYYY-MM-DD strings).
func (s *Store) PublishStatsByDate(ctx context.Context, from, to string) ([]DayPublishStat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.post_date,
		       SUM(CASE WHEN pt.status='published' THEN 1 ELSE 0 END) AS published,
		       SUM(CASE WHEN pt.status='published' AND pt.tier IN ('blog','linkedin') THEN 1 ELSE 0 END) AS credibility
		FROM posts p
		JOIN post_tiers pt ON pt.post_id = p.id
		WHERE p.post_date BETWEEN ? AND ?
		GROUP BY p.post_date
		ORDER BY p.post_date`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayPublishStat
	for rows.Next() {
		var d DayPublishStat
		if err := rows.Scan(&d.Date, &d.Published, &d.Credibility); err != nil {
			return nil, err
		}
		d.Date = dateOnly(d.Date)
		out = append(out, d)
	}
	return out, rows.Err()
}

// PublishedTierCounts returns the count of published tiers per tier within
// [from, to]. Tiers with zero published in the window are omitted.
func (s *Store) PublishedTierCounts(ctx context.Context, from, to string) (map[domain.Tier]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pt.tier, COUNT(*)
		FROM posts p
		JOIN post_tiers pt ON pt.post_id = p.id
		WHERE pt.status='published' AND p.post_date BETWEEN ? AND ?
		GROUP BY pt.tier`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.Tier]int{}
	for rows.Next() {
		var t domain.Tier
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rows.Err()
}
