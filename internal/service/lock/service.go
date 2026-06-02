package lock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Flashgap/marvin/pkg/database"
	"github.com/Flashgap/marvin/pkg/logger"
	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
)

// cooldownWindow caps how often the same (victim, finder) pair may generate
// new point swings. Mainly a sanity guard against accidental double-submits.
const cooldownWindow = 10 * time.Minute

// mentionRE matches Slack's escaped user-mention forms: "<@U12345>" or
// "<@U12345|alice>". Captures the ID and (optional) inline name.
var mentionRE = regexp.MustCompile(`^<@([UW][A-Z0-9]+)(?:\|([^>]*))?>$`)

type service struct {
	db       database.Client
	slack    slacksvc.Service
	queries  queries
}

// NewService applies pending migrations and wires the lock service. A non-nil
// db is required; pass migrations via mfs (typically internal/migrations.FS).
func NewService(ctx context.Context, db database.Client, slack slacksvc.Service, mfs fs.FS) (Service, error) {
	if err := db.Migrate(ctx, mfs); err != nil {
		return nil, fmt.Errorf("lock: applying migrations: %w", err)
	}
	return newServiceUnchecked(db, slack), nil
}

// NewTestService builds a Service without running migrations. Use only from
// tests that drive the database via sqlmock.
func NewTestService(db database.Client, slack slacksvc.Service) Service {
	return newServiceUnchecked(db, slack)
}

func newServiceUnchecked(db database.Client, slack slacksvc.Service) Service {
	return &service{
		db:      db,
		slack:   slack,
		queries: buildQueries(db.Dialect()),
	}
}

func (s *service) Lock(ctx context.Context, payload SlashPayload) (*Response, error) {
	log := logger.WithContext(ctx).WithPrefix("[lock.Lock]")

	finderID, finderInlineName, ok := parseMention(payload.Text)
	if !ok {
		return ephemeral("Usage: `/lock @someone` (your Slack app must have *Escape channels, users, and links* enabled)."), nil
	}

	victimID := payload.UserID
	if victimID == finderID {
		return ephemeral("You can't lock yourself."), nil
	}

	// Look up the finder so we can both reject bots and grab a canonical name.
	finderUser, err := s.slack.GetUser(ctx, finderID)
	if err != nil {
		return nil, fmt.Errorf("lookup finder: %w", err)
	}
	if finderUser.IsBot {
		return ephemeral("Bots don't have laptops."), nil
	}
	finderName := finderInlineName
	if finderName == "" {
		finderName = finderUser.Name
	}

	// Resolve victim name. Form payload normally has it; fall back to a lookup.
	victimName := payload.UserName
	if victimName == "" {
		v, err := s.slack.GetUser(ctx, victimID)
		if err != nil {
			return nil, fmt.Errorf("lookup victim: %w", err)
		}
		victimName = v.Name
	}

	// Cooldown.
	cutoff := time.Now().Add(-cooldownWindow)
	var x int
	row := s.db.DB().QueryRowContext(ctx, s.queries.recentEvent, victimID, finderID, cutoff)
	switch err := row.Scan(&x); {
	case err == nil:
		return ephemeral("You just got locked by this person. Give it a moment."), nil
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("cooldown check: %w", err)
	}

	// Apply the point change in a single transaction.
	victimPts, finderPts, err := s.applyLock(ctx, victimID, victimName, finderID, finderName)
	if err != nil {
		return nil, err
	}

	// Fire-and-forget DM to the victim. Use a detached context so a slow
	// PostMessage doesn't block (or cancel with) the HTTP response.
	go func() { //nolint:gosec,contextcheck // detached context is intentional: HTTP request ctx is canceled when the slash command response is written, but the DM may still need to send
		dmCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg := fmt.Sprintf("<@%s> found your laptop unlocked and locked it for you. You're now at %d points.", finderID, victimPts)
		if err := s.slack.SendDM(dmCtx, victimID, msg); err != nil {
			log.Warnf("failed to DM victim %s: %v", victimID, err)
		}
	}()

	return ephemeral(fmt.Sprintf("Locked. <@%s>: %d • you: %d", finderID, finderPts, victimPts)), nil
}

func (s *service) applyLock(ctx context.Context, victimID, victimName, finderID, finderName string) (victimPts, finderPts int, err error) {
	tx, err := s.db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, s.queries.upsertUser, victimID, victimName, -1); err != nil {
		return 0, 0, fmt.Errorf("upsert victim: %w", err)
	}
	if _, err := tx.ExecContext(ctx, s.queries.upsertUser, finderID, finderName, 1); err != nil {
		return 0, 0, fmt.Errorf("upsert finder: %w", err)
	}
	if _, err := tx.ExecContext(ctx, s.queries.insertEvent, victimID, finderID); err != nil {
		return 0, 0, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit: %w", err)
	}

	// Read the resulting tallies. Separate queries (post-commit) keep the
	// transaction tight and avoid driver-specific RETURNING vs LAST_INSERT_ID.
	if err := s.db.DB().QueryRowContext(ctx, s.pointsQuery(), victimID).Scan(&victimPts); err != nil {
		return 0, 0, fmt.Errorf("read victim points: %w", err)
	}
	if err := s.db.DB().QueryRowContext(ctx, s.pointsQuery(), finderID).Scan(&finderPts); err != nil {
		return 0, 0, fmt.Errorf("read finder points: %w", err)
	}
	return victimPts, finderPts, nil
}

func (s *service) pointsQuery() string {
	return "SELECT points FROM lock_users WHERE slack_user_id = " + s.db.Dialect().Placeholder(1)
}

func (s *service) Leaderboard(ctx context.Context) (*Response, error) {
	top, err := s.queryLeaderboard(ctx, s.queries.topUsers)
	if err != nil {
		return nil, err
	}
	bottom, err := s.queryLeaderboard(ctx, s.queries.bottomUsers)
	if err != nil {
		return nil, err
	}

	if len(top) == 0 && len(bottom) == 0 {
		return ephemeral("No locks yet. Watch your laptops."), nil
	}

	var b strings.Builder
	b.WriteString("*Top 3*\n")
	writeLeaderboard(&b, top)
	b.WriteString("\n*Bottom 3*\n")
	writeLeaderboard(&b, bottom)
	return ephemeral(b.String()), nil
}

type leaderboardRow struct {
	UserID string
	Name   string
	Points int
}

func (s *service) queryLeaderboard(ctx context.Context, q string) ([]leaderboardRow, error) {
	rows, err := s.db.DB().QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("leaderboard query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []leaderboardRow
	for rows.Next() {
		var r leaderboardRow
		if err := rows.Scan(&r.UserID, &r.Name, &r.Points); err != nil {
			return nil, fmt.Errorf("leaderboard scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func writeLeaderboard(b *strings.Builder, rows []leaderboardRow) {
	if len(rows) == 0 {
		b.WriteString("_(empty)_\n")
		return
	}
	// Stable ordering when points are tied — sort by user_id ascending.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Points != rows[j].Points {
			return false // preserve query order on points
		}
		return rows[i].UserID < rows[j].UserID
	})
	for i, r := range rows {
		name := r.Name
		if name == "" {
			name = r.UserID
		}
		fmt.Fprintf(b, "%d. @%s — %d\n", i+1, name, r.Points)
	}
}

func parseMention(text string) (id, inlineName string, ok bool) {
	m := mentionRE.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func ephemeral(text string) *Response {
	return &Response{Type: ResponseEphemeral, Text: text}
}
