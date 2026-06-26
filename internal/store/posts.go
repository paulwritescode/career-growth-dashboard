package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

const postCols = `id, sprint_id, source_log_id, post_date, post_type, title, is_declaration, created_at, updated_at`

func scanPost(row interface{ Scan(...any) error }) (domain.Post, error) {
	var p domain.Post
	var (
		sprintID, sourceLogID sql.NullInt64
		isDecl                int
		createdAt, updatedAt  string
	)
	if err := row.Scan(&p.ID, &sprintID, &sourceLogID, &p.PostDate, &p.PostType,
		&p.Title, &isDecl, &createdAt, &updatedAt); err != nil {
		return domain.Post{}, err
	}
	p.SprintID = intFromNull(sprintID)
	p.SourceLogID = intFromNull(sourceLogID)
	p.PostDate = dateOnly(p.PostDate)
	p.IsDeclaration = isDecl == 1
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	return p, nil
}

// CreatePost inserts a post plus its three tiers (one per domain.AllTiers) in a
// single transaction and returns the post with tiers populated.
func (s *Store) CreatePost(ctx context.Context, in domain.Post) (domain.Post, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Post{}, err
	}
	now := nowUTC()
	res, err := tx.ExecContext(ctx, `INSERT INTO posts
		(sprint_id, source_log_id, post_date, post_type, title, is_declaration, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		nullInt(in.SprintID), nullInt(in.SourceLogID), in.PostDate, in.PostType,
		in.Title, b2i(in.IsDeclaration), now, now)
	if err != nil {
		_ = tx.Rollback()
		return domain.Post{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return domain.Post{}, err
	}
	for _, tier := range domain.AllTiers() {
		if _, err := tx.ExecContext(ctx, `INSERT INTO post_tiers
			(post_id, tier, status, visual_kind, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
			id, tier, domain.TierNone, domain.VisualNone, now, now); err != nil {
			_ = tx.Rollback()
			return domain.Post{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Post{}, err
	}
	return s.GetPost(ctx, id)
}

// GetPost returns a post with its tiers populated, or ErrNotFound.
func (s *Store) GetPost(ctx context.Context, id int64) (domain.Post, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+postCols+` FROM posts WHERE id = ?`, id)
	p, err := scanPost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Post{}, ErrNotFound
	}
	if err != nil {
		return domain.Post{}, err
	}
	tiers, err := s.ListTiers(ctx, p.ID)
	if err != nil {
		return domain.Post{}, err
	}
	p.Tiers = tiers
	return p, nil
}

// GetPostByDate returns the post for a local date with tiers, or ErrNotFound.
func (s *Store) GetPostByDate(ctx context.Context, date string) (domain.Post, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+postCols+` FROM posts WHERE post_date = ?`, date)
	p, err := scanPost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Post{}, ErrNotFound
	}
	if err != nil {
		return domain.Post{}, err
	}
	tiers, err := s.ListTiers(ctx, p.ID)
	if err != nil {
		return domain.Post{}, err
	}
	p.Tiers = tiers
	return p, nil
}

// ListPosts returns posts (without tiers) newest first, up to limit.
func (s *Store) ListPosts(ctx context.Context, limit int) ([]domain.Post, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+postCols+` FROM posts ORDER BY post_date DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdatePost persists the mutable header fields of a post.
func (s *Store) UpdatePost(ctx context.Context, p domain.Post) error {
	_, err := s.db.ExecContext(ctx, `UPDATE posts SET
		sprint_id=?, source_log_id=?, post_date=?, post_type=?, title=?, is_declaration=?, updated_at=?
		WHERE id=?`,
		nullInt(p.SprintID), nullInt(p.SourceLogID), p.PostDate, p.PostType,
		p.Title, b2i(p.IsDeclaration), nowUTC(), p.ID)
	return err
}

// --- post tiers -----------------------------------------------------------

const tierCols = `id, post_id, tier, status, content, url, published_at, visual_kind, adr_id, created_at, updated_at`

func scanTier(row interface{ Scan(...any) error }) (domain.PostTier, error) {
	var t domain.PostTier
	var (
		url, publishedAt     sql.NullString
		adrID                sql.NullInt64
		createdAt, updatedAt string
	)
	if err := row.Scan(&t.ID, &t.PostID, &t.Tier, &t.Status, &t.Content, &url,
		&publishedAt, &t.VisualKind, &adrID, &createdAt, &updatedAt); err != nil {
		return domain.PostTier{}, err
	}
	t.URL = strFromNull(url)
	t.PublishedAt = timeFromNull(publishedAt)
	t.ADRID = intFromNull(adrID)
	t.CreatedAt = parseTime(createdAt)
	t.UpdatedAt = parseTime(updatedAt)
	return t, nil
}

// ListTiers returns the three tiers of a post in canonical order.
func (s *Store) ListTiers(ctx context.Context, postID int64) ([]domain.PostTier, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tierCols+` FROM post_tiers WHERE post_id = ?
		ORDER BY CASE tier WHEN 'blog' THEN 0 WHEN 'linkedin' THEN 1 ELSE 2 END`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PostTier
	for rows.Next() {
		t, err := scanTier(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateTier persists all mutable fields of a single tier.
func (s *Store) UpdateTier(ctx context.Context, t domain.PostTier) error {
	var publishedAt any
	if t.PublishedAt != nil {
		publishedAt = t.PublishedAt.UTC().Format(timeLayout)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE post_tiers SET
		status=?, content=?, url=?, published_at=?, visual_kind=?, adr_id=?, updated_at=?
		WHERE post_id=? AND tier=?`,
		t.Status, t.Content, nullStr(t.URL), publishedAt, t.VisualKind, nullInt(t.ADRID),
		nowUTC(), t.PostID, t.Tier)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetTier returns a single tier of a post.
func (s *Store) GetTier(ctx context.Context, postID int64, tier domain.Tier) (domain.PostTier, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+tierCols+` FROM post_tiers WHERE post_id=? AND tier=?`, postID, tier)
	t, err := scanTier(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PostTier{}, ErrNotFound
	}
	return t, err
}
