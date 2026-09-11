package updates

import (
	"testing"
	"time"
)

func TestBuildViewCountsUnreadItemsAndPreservesOrder(t *testing.T) {
	published := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	items := []Item{
		{ID: "new", PublishedAt: published, Read: false},
		{ID: "old", PublishedAt: published.Add(-time.Hour), Read: true},
		{ID: "newer", PublishedAt: published.Add(time.Hour), Read: false},
	}

	view := BuildView(items)
	if view.UnreadCount != 2 {
		t.Fatalf("UnreadCount = %d, want 2", view.UnreadCount)
	}
	if len(view.Items) != len(items) || view.Items[0].ID != "new" || view.Items[2].ID != "newer" {
		t.Fatalf("BuildView changed item order: %#v", view.Items)
	}
}

func TestLocalizeSupportsAllLanguagesAndSerbianFallback(t *testing.T) {
	translations := []struct {
		lang string
		want string
	}{
		{lang: "sr", want: "Српски"},
		{lang: "en", want: "English"},
		{lang: "ru", want: "Русский"},
		{lang: "de", want: "Српски"},
	}
	for _, tc := range translations {
		t.Run(tc.lang, func(t *testing.T) {
			got := Localize(tc.lang, "Српски", "English", "Русский")
			if got != tc.want {
				t.Fatalf("Localize(%q) = %q, want %q", tc.lang, got, tc.want)
			}
		})
	}

	if got := Localize("en", "Српски", "", "Русский"); got != "Српски" {
		t.Fatalf("empty English translation fallback = %q, want Serbian", got)
	}
}

func TestPublishedAfterRegistrationIsStrict(t *testing.T) {
	registered := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	if PublishedAfterRegistration(registered, registered) {
		t.Fatal("an update published at registration time must not be visible")
	}
	if PublishedAfterRegistration(registered.Add(-time.Nanosecond), registered) {
		t.Fatal("an update published before registration must not be visible")
	}
	if !PublishedAfterRegistration(registered.Add(time.Nanosecond), registered) {
		t.Fatal("an update published after registration must be visible")
	}
}

func TestMarkReadRejectsInvalidUserTypeWithoutDatabaseAccess(t *testing.T) {
	repo := NewRepo(nil)
	if err := repo.MarkRead(nil, "invalid", "not-a-user", []string{"not-an-update"}); err == nil {
		t.Fatal("MarkRead accepted an invalid user type")
	}
}
