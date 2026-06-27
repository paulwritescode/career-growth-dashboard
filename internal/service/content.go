package service

import (
	"context"
	"fmt"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// CreatePostInput describes a new day's content update.
type CreatePostInput struct {
	PostDate      string // defaults to today
	PostType      domain.PostType
	SprintID      *int64
	SourceLogID   *int64
	Title         string
	IsDeclaration bool
	Source        domain.EventSource
}

// CreatePost creates a post for a day with its three (none-status) tiers. If a
// post already exists for the date it is returned as-is (one post per day).
func (s *Service) CreatePost(ctx context.Context, in CreatePostInput) (domain.Post, error) {
	if in.PostDate == "" {
		in.PostDate = s.Today()
	}
	if in.PostType == "" {
		in.PostType = domain.PostDaily
	}
	if !in.PostType.Valid() {
		return domain.Post{}, validationf("invalid post type %q", in.PostType)
	}
	if existing, err := s.store.GetPostByDate(ctx, in.PostDate); err == nil {
		return existing, nil
	} else if err != ErrNotFound {
		return domain.Post{}, err
	}

	// Default the sprint to the current active one when not specified.
	if in.SprintID == nil {
		if cur, err := s.store.GetCurrentSprint(ctx); err == nil {
			in.SprintID = &cur.ID
		}
	}

	post, err := s.store.CreatePost(ctx, domain.Post{
		SprintID:      in.SprintID,
		SourceLogID:   in.SourceLogID,
		PostDate:      in.PostDate,
		PostType:      in.PostType,
		Title:         trim(in.Title),
		IsDeclaration: in.IsDeclaration,
	})
	if err != nil {
		return domain.Post{}, err
	}
	s.appendEvent(ctx, "post.created", in.Source, in.SprintID, &post.ID,
		"Created "+string(post.PostType)+" post for "+post.PostDate, "")

	// Declaration back-link (spec 04/05): a Phase-1 declaration post links back
	// to its sprint via sprints.declaration_post_id so the sprint knows which
	// post announced it.
	if post.IsDeclaration && post.SprintID != nil {
		if sp, err := s.store.GetSprint(ctx, *post.SprintID); err == nil {
			sp.DeclarationPostID = &post.ID
			if err := s.store.UpdateSprint(ctx, sp); err != nil {
				return domain.Post{}, err
			}
			s.appendEvent(ctx, "sprint.declared", in.Source, post.SprintID, &post.ID,
				sp.SkillName+": declaration post linked", "")
		}
	}
	return post, nil
}

// GetPost returns a post with its tiers.
func (s *Service) GetPost(ctx context.Context, id int64) (domain.Post, error) {
	return s.store.GetPost(ctx, id)
}

// PostByDate returns the post for a date with tiers, or ErrNotFound.
func (s *Service) PostByDate(ctx context.Context, date string) (domain.Post, error) {
	return s.store.GetPostByDate(ctx, date)
}

// ListPosts returns recent posts (without tiers).
func (s *Service) ListPosts(ctx context.Context, limit int) ([]domain.Post, error) {
	return s.store.ListPosts(ctx, limit)
}

// DraftTier saves drafted content for a tier without publishing it. Publishing
// is a separate, explicit action (MarkPublished).
func (s *Service) DraftTier(ctx context.Context, postID int64, tier domain.Tier, content string, src domain.EventSource) (domain.PostTier, error) {
	if !tier.Valid() {
		return domain.PostTier{}, validationf("invalid tier %q", tier)
	}
	t, err := s.store.GetTier(ctx, postID, tier)
	if err != nil {
		return domain.PostTier{}, err
	}
	t.Content = content
	if t.Status == domain.TierNone {
		t.Status = domain.TierDrafted
	}
	if err := s.store.UpdateTier(ctx, t); err != nil {
		return domain.PostTier{}, err
	}
	s.appendEvent(ctx, "post.tier_drafted", src, nil, &postID, "Drafted "+tier.Label()+" tier", "")
	return s.store.GetTier(ctx, postID, tier)
}

// MarkPublished marks a tier as published. Enforces the skill rule that a
// published tier must have a URL.
func (s *Service) MarkPublished(ctx context.Context, postID int64, tier domain.Tier, url string, src domain.EventSource) (domain.PostTier, error) {
	if !tier.Valid() {
		return domain.PostTier{}, validationf("invalid tier %q", tier)
	}
	if trim(url) == "" {
		return domain.PostTier{}, ErrURLRequired
	}
	t, err := s.store.GetTier(ctx, postID, tier)
	if err != nil {
		return domain.PostTier{}, err
	}
	u := trim(url)
	now := s.now().UTC()
	t.Status = domain.TierPublished
	t.URL = &u
	t.PublishedAt = &now
	if err := s.store.UpdateTier(ctx, t); err != nil {
		return domain.PostTier{}, err
	}
	s.appendEvent(ctx, "post.published", src, nil, &postID, "Published "+tier.Label()+" → "+u, "")
	return s.store.GetTier(ctx, postID, tier)
}

// SetTierVisual sets the LinkedIn-tier visual attachment choice (the skill
// nudges toward an ADR or animated diagram over a flat screenshot).
func (s *Service) SetTierVisual(ctx context.Context, postID int64, tier domain.Tier, kind domain.VisualKind, adrID *int64, src domain.EventSource) (domain.PostTier, error) {
	if !kind.Valid() {
		return domain.PostTier{}, validationf("invalid visual kind %q", kind)
	}
	t, err := s.store.GetTier(ctx, postID, tier)
	if err != nil {
		return domain.PostTier{}, err
	}
	t.VisualKind = kind
	if kind == domain.VisualADR {
		t.ADRID = adrID
	} else {
		t.ADRID = nil
	}
	if err := s.store.UpdateTier(ctx, t); err != nil {
		return domain.PostTier{}, err
	}
	s.appendEvent(ctx, "post.visual_set", src, nil, &postID, tier.Label()+" visual → "+string(kind), "")
	return s.store.GetTier(ctx, postID, tier)
}

// CreateADRInput mirrors adr-lite-template.md.
type CreateADRInput struct {
	Number       int // 0 => auto-assign next
	Title        string
	Status       domain.ADRStatus
	DecidedOn    string
	Problem      string
	Options      string
	Decision     string
	Why          string
	Consequences string
	SprintID     *int64
	Source       domain.EventSource
}

// CreateADR creates an Architecture Decision Record, auto-assigning the next
// number when none is given.
func (s *Service) CreateADR(ctx context.Context, in CreateADRInput) (domain.ADR, error) {
	if trim(in.Title) == "" {
		return domain.ADR{}, validationf("ADR title is required")
	}
	if in.Status == "" {
		in.Status = domain.ADRProposed
	}
	if !in.Status.Valid() {
		return domain.ADR{}, validationf("invalid ADR status %q", in.Status)
	}
	if in.Number <= 0 {
		max, err := s.store.MaxADRNumber(ctx)
		if err != nil {
			return domain.ADR{}, err
		}
		in.Number = max + 1
	}
	var decidedOn *string
	if d := trim(in.DecidedOn); d != "" {
		decidedOn = &d
	}
	adr, err := s.store.CreateADR(ctx, domain.ADR{
		SprintID: in.SprintID, Number: in.Number, Title: trim(in.Title), Status: in.Status,
		DecidedOn: decidedOn, Problem: in.Problem, Options: in.Options, Decision: in.Decision,
		Why: in.Why, Consequences: in.Consequences,
	})
	if err != nil {
		return domain.ADR{}, err
	}
	s.appendEvent(ctx, "adr.created", in.Source, in.SprintID, nil,
		fmt.Sprintf("Created ADR-%d: %s", adr.Number, adr.Title), "")
	return adr, nil
}

// GetADR returns an ADR by ID.
func (s *Service) GetADR(ctx context.Context, id int64) (domain.ADR, error) {
	return s.store.GetADR(ctx, id)
}

// ListADRs returns all ADRs, newest first.
func (s *Service) ListADRs(ctx context.Context) ([]domain.ADR, error) {
	return s.store.ListADRs(ctx)
}

// DeletePost removes a post and its tiers. Records a career event.
func (s *Service) DeletePost(ctx context.Context, id int64, src domain.EventSource) error {
	post, err := s.store.GetPost(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeletePost(ctx, id); err != nil {
		return err
	}
	s.appendEvent(ctx, "post.deleted", src, post.SprintID, &id, "Deleted post for "+post.PostDate, "")
	return nil
}

// UpdateADR updates an existing ADR's fields. Records a career event.
func (s *Service) UpdateADR(ctx context.Context, id int64, in CreateADRInput) (domain.ADR, error) {
	adr, err := s.store.GetADR(ctx, id)
	if err != nil {
		return domain.ADR{}, err
	}
	if trim(in.Title) != "" {
		adr.Title = trim(in.Title)
	}
	if in.Status != "" && in.Status.Valid() {
		adr.Status = in.Status
	}
	if trim(in.DecidedOn) != "" {
		d := trim(in.DecidedOn)
		adr.DecidedOn = &d
	}
	if in.Problem != "" {
		adr.Problem = in.Problem
	}
	if in.Options != "" {
		adr.Options = in.Options
	}
	if in.Decision != "" {
		adr.Decision = in.Decision
	}
	if in.Why != "" {
		adr.Why = in.Why
	}
	if in.Consequences != "" {
		adr.Consequences = in.Consequences
	}
	if in.SprintID != nil {
		adr.SprintID = in.SprintID
	}
	if err := s.store.UpdateADR(ctx, adr); err != nil {
		return domain.ADR{}, err
	}
	s.appendEvent(ctx, "adr.updated", in.Source, adr.SprintID, nil,
		fmt.Sprintf("Updated ADR-%d: %s", adr.Number, adr.Title), "")
	return s.store.GetADR(ctx, id)
}

// DeleteADR removes an ADR. Records a career event.
func (s *Service) DeleteADR(ctx context.Context, id int64, src domain.EventSource) error {
	adr, err := s.store.GetADR(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteADR(ctx, id); err != nil {
		return err
	}
	s.appendEvent(ctx, "adr.deleted", src, adr.SprintID, nil,
		fmt.Sprintf("Deleted ADR-%d: %s", adr.Number, adr.Title), "")
	return nil
}
