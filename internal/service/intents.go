package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// RunChatIntent executes an allowlisted mutation requested by the kiro-cli chat
// bridge. It is the single, validated door through which the agent changes
// data — the agent never touches SQL or the store directly (specs 07 & 10).
// Every mutation is attributed to source="chat" in the career-event audit
// trail, so chat-driven and form-driven changes are indistinguishable in state
// but distinguishable in provenance.
//
// It returns a short human-readable confirmation on success. Unknown intents
// return an error and are never executed.
func (s *Service) RunChatIntent(ctx context.Context, name string, fields map[string]string) (string, error) {
	src := domain.SourceChat
	get := func(k string) string { return strings.TrimSpace(fields[k]) }

	switch name {
	case "log.record":
		in := RecordLogInput{
			WorkedOn:     get("worked_on"),
			WhatHappened: get("what_happened"),
			Insight:      get("insight"),
			NextUp:       get("next_up"),
			Blocker:      get("blocker"),
			LogDate:      get("log_date"),
			Source:       src,
		}
		if bd := get("blocker_decision"); bd != "" {
			d := domain.BlockerDecision(bd)
			in.BlockerDecision = &d
		}
		log, err := s.RecordLog(ctx, in)
		if err != nil {
			return "", err
		}
		return "logged: " + log.WorkedOn, nil

	case "post.create":
		post, err := s.CreatePost(ctx, CreatePostInput{
			PostType:      domain.PostType(orDefault(get("post_type"), string(domain.PostDaily))),
			Title:         get("title"),
			PostDate:      get("post_date"),
			IsDeclaration: isTrue(get("is_declaration")),
			Source:        src,
		})
		if err != nil {
			return "", err
		}
		return "created " + string(post.PostType) + " post for " + post.PostDate, nil

	case "post.draft":
		tier := domain.Tier(get("tier"))
		post, err := s.ensureTodayPost(ctx, src)
		if err != nil {
			return "", err
		}
		if _, err := s.DraftTier(ctx, post.ID, tier, get("content"), src); err != nil {
			return "", err
		}
		return "drafted " + tier.Label() + " tier for " + post.PostDate, nil

	case "post.mark_published":
		tier := domain.Tier(get("tier"))
		post, err := s.ensureTodayPost(ctx, src)
		if err != nil {
			return "", err
		}
		if _, err := s.MarkPublished(ctx, post.ID, tier, get("url"), src); err != nil {
			return "", err
		}
		return "published " + tier.Label() + " → " + get("url"), nil

	case "sprint.create":
		sp, err := s.CreateSprint(ctx, CreateSprintInput{
			SkillName:        get("skill_name"),
			MicroappOneLiner: get("microapp_one_liner"),
			CoreFeature:      get("core_feature"),
			SkillRationale:   get("skill_rationale"),
			OutOfScope:       get("out_of_scope"),
			DeployPlatform:   get("deploy_platform"),
			MonthLabel:       get("month_label"),
			Status:           domain.SprintStatus(get("status")),
			Source:           src,
		})
		if err != nil {
			return "", err
		}
		return "started sprint: " + sp.SkillName + " — " + sp.MicroappOneLiner, nil

	case "sprint.set_phase":
		cur, err := s.store.GetCurrentSprint(ctx)
		if err == ErrNotFound {
			return "", validationf("no active sprint to move")
		} else if err != nil {
			return "", err
		}
		n, err := strconv.Atoi(get("phase"))
		if err != nil {
			return "", validationf("phase must be a number 1-4")
		}
		sp, err := s.SetPhase(ctx, cur.ID, domain.Phase(n), src)
		if err != nil {
			return "", err
		}
		return sp.SkillName + ": moved to phase " + sp.CurrentPhase.Label(), nil

	case "sprint.ship":
		cur, err := s.store.GetCurrentSprint(ctx)
		if err == ErrNotFound {
			return "", validationf("no active sprint to ship")
		} else if err != nil {
			return "", err
		}
		sp, err := s.SetStatus(ctx, cur.ID, SetStatusInput{
			Status: domain.SprintShipped, LiveURL: get("live_url"), Source: src,
		})
		if err != nil {
			return "", err
		}
		return "shipped " + sp.SkillName, nil

	case "adr.create":
		adr, err := s.CreateADR(ctx, CreateADRInput{
			Title:        get("title"),
			Problem:      get("problem"),
			Options:      get("options"),
			Decision:     get("decision"),
			Why:          get("why"),
			Consequences: get("consequences"),
			Status:       domain.ADRStatus(get("status")),
			DecidedOn:    get("decided_on"),
			Source:       src,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("created ADR-%d: %s", adr.Number, adr.Title), nil

	default:
		return "", fmt.Errorf("unsupported intent %q", name)
	}
}

// ensureTodayPost returns today's post, creating it (with its three tiers) if
// none exists yet — so "publish today's LinkedIn post" works even before a post
// row was made.
func (s *Service) ensureTodayPost(ctx context.Context, src domain.EventSource) (domain.Post, error) {
	if p, err := s.store.GetPostByDate(ctx, s.Today()); err == nil {
		return p, nil
	} else if err != ErrNotFound {
		return domain.Post{}, err
	}
	return s.CreatePost(ctx, CreatePostInput{Source: src})
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func isTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}
