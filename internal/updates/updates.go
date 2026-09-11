// Package updates provides the authenticated in-app updates feed.
package updates

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"time"

	"github.com/lib/pq"

	"buh/internal/auth"
)

// Item is one localized update visible to a user.
type Item struct {
	ID          string
	Title       string
	Body        template.HTML
	PublishedAt time.Time
	Read        bool
}

// View is the updates feed and its current unread count.
type View struct {
	Items       []Item
	UnreadCount int
}

// BuildView calculates the presentation metadata without accessing a database.
func BuildView(rows []Item) View {
	view := View{Items: rows}
	for _, item := range rows {
		if !item.Read {
			view.UnreadCount++
		}
	}
	return view
}

// Localize selects the requested translation and falls back to Serbian when
// the requested language is unsupported or its translation is empty.
func Localize(lang, sr, en, ru string) string {
	switch lang {
	case "en":
		if en != "" {
			return en
		}
	case "ru":
		if ru != "" {
			return ru
		}
	}
	return sr
}

// PublishedAfterRegistration is the visibility rule for an update. Equality
// is excluded because an update must be published after registration.
func PublishedAfterRegistration(publishedAt, registeredAt time.Time) bool {
	return publishedAt.After(registeredAt)
}

// Repo persists updates and per-user read markers.
type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// View returns updates published strictly after the user's registration.
// The user type is validated before it is used to select one of the two known
// account tables; it is never interpolated from an arbitrary request value.
func (r *Repo) View(ctx context.Context, userType auth.UserType, userID, lang string) (View, error) {
	if userType != auth.UserTypeAccountant && userType != auth.UserTypeEntrepreneur {
		return View{}, fmt.Errorf("updates: invalid user type %q", userType)
	}

	userTable := "accountants"
	if userType == auth.UserTypeEntrepreneur {
		userTable = "entrepreneur_users"
	}

	query := fmt.Sprintf(`
		SELECT u.id, u.title_sr, u.body_sr, u.title_en, u.body_en,
		       u.title_ru, u.body_ru, u.published_at,
		       (r.update_id IS NOT NULL)
		FROM app_updates u
		LEFT JOIN app_update_reads r
		  ON r.update_id = u.id
		 AND r.user_type = $1
		 AND r.user_id = $2::uuid
		WHERE u.published_at > (
			SELECT created_at FROM %s WHERE id = $2::uuid
		)
		ORDER BY u.published_at DESC, u.id DESC`, userTable)

	rows, err := r.db.QueryContext(ctx, query, string(userType), userID)
	if err != nil {
		return View{}, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		var titleSR, bodySR, titleEN, bodyEN, titleRU, bodyRU string
		if err := rows.Scan(
			&item.ID,
			&titleSR, &bodySR,
			&titleEN, &bodyEN,
			&titleRU, &bodyRU,
			&item.PublishedAt,
			&item.Read,
		); err != nil {
			return View{}, err
		}
		item.Title = Localize(lang, titleSR, titleEN, titleRU)
		// Update bodies are developer-authored HTML from migrations. Keep the
		// trusted type through rendering so semantic markup is not escaped.
		item.Body = template.HTML(Localize(lang, bodySR, bodyEN, bodyRU))
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return View{}, err
	}
	return BuildView(items), nil
}

// MarkRead marks the displayed updates as read for this user. The query repeats
// the registration cutoff so callers cannot mark an update from before signup.
// Repeated calls are safe.
func (r *Repo) MarkRead(ctx context.Context, userType auth.UserType, userID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if userType != auth.UserTypeAccountant && userType != auth.UserTypeEntrepreneur {
		return fmt.Errorf("updates: invalid user type %q", userType)
	}

	userTable := "accountants"
	if userType == auth.UserTypeEntrepreneur {
		userTable = "entrepreneur_users"
	}

	query := fmt.Sprintf(`
		INSERT INTO app_update_reads (update_id, user_type, user_id)
		SELECT u.id, $1, $2::uuid
		FROM app_updates u
		WHERE u.id = ANY($3::uuid[])
		  AND u.published_at > (
			SELECT created_at FROM %s WHERE id = $2::uuid
		  )
		ON CONFLICT (update_id, user_type, user_id) DO NOTHING`, userTable)
	_, err := r.db.ExecContext(ctx, query, string(userType), userID, pq.Array(ids))
	return err
}
