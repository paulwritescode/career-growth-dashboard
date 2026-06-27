package service

import (
	"context"
	"testing"
	"time"

	"github.com/paulkinyatti/local-scava/internal/domain"
)

// TestRunChatIntentLogRecord verifies a chat intent writes through the service
// and is attributed to source="chat" in the audit trail.
func TestRunChatIntentLogRecord(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	mustSprint(t, svc)

	summary, err := svc.RunChatIntent(ctx, "log.record", map[string]string{
		"worked_on": "wired the chat intent bridge",
		"insight":   "agent emits SCAVA-ACTION lines",
	})
	if err != nil {
		t.Fatalf("RunChatIntent log.record: %v", err)
	}
	if summary == "" {
		t.Fatal("expected a non-empty confirmation summary")
	}

	logs, _ := svc.ListLogs(ctx, 10)
	if len(logs) != 1 || logs[0].WorkedOn != "wired the chat intent bridge" {
		t.Fatalf("log not recorded via intent: %+v", logs)
	}

	events, _ := svc.ListEvents(ctx, 10)
	var chatEvent bool
	for _, e := range events {
		if e.Source == domain.SourceChat {
			chatEvent = true
		}
	}
	if !chatEvent {
		t.Fatal("expected a career event with source=chat")
	}
}

// TestRunChatIntentPublishCreatesTodayPost verifies post.mark_published works
// even when no post exists yet (it creates today's post first).
func TestRunChatIntentPublishCreatesTodayPost(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))

	if _, err := svc.RunChatIntent(ctx, "post.mark_published", map[string]string{
		"tier": "linkedin", "url": "https://linkedin.com/p/1",
	}); err != nil {
		t.Fatalf("RunChatIntent post.mark_published: %v", err)
	}
	post, err := svc.PostByDate(ctx, svc.Today())
	if err != nil {
		t.Fatalf("expected today's post to exist: %v", err)
	}
	var published bool
	for _, tr := range post.Tiers {
		if tr.Tier == domain.TierLinkedIn && tr.IsPublished() {
			published = true
		}
	}
	if !published {
		t.Fatalf("linkedin tier not published: %+v", post.Tiers)
	}
}

// TestRunChatIntentUnknownRejected verifies unknown intents are never executed.
func TestRunChatIntentUnknownRejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t, time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC))
	if _, err := svc.RunChatIntent(ctx, "sprint.delete_all", map[string]string{}); err == nil {
		t.Fatal("expected unknown intent to be rejected")
	}
}
