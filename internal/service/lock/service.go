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

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/slack-go/slack"

	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	"github.com/Flashgap/marvin/pkg/database"
	"github.com/Flashgap/marvin/pkg/logger"
)

// cooldownWindow caps how often the same (victim, finder) pair may generate
// new point swings. Mainly a sanity guard against accidental double-submits.
const cooldownWindow = 10 * time.Minute

// mentionRE matches Slack's escaped user-mention forms: "<@U12345>" or
// "<@U12345|alice>". Captures the ID and (optional) inline name.
var mentionRE = regexp.MustCompile(`^<@([UW][A-Z0-9]+)(?:\|([^>]*))?>$`)

type service struct {
	db    database.Client
	slack slacksvc.Service
}

// NewService applies pending migrations and wires the lock service. A non-nil
// db is required; pass migrations via mfs (typically internal/migrations.FS).
func NewService(ctx context.Context, db database.Client, slackSvc slacksvc.Service, mfs fs.FS) (Service, error) {
	if db == nil {
		return nil, fmt.Errorf("lock: database client is required")
	}
	if slackSvc == nil {
		return nil, fmt.Errorf("lock: slack service is required")
	}
	if err := db.Migrate(ctx, mfs); err != nil {
		return nil, fmt.Errorf("lock: applying migrations: %w", err)
	}
	return newServiceUnchecked(db, slackSvc), nil
}

// NewTestService builds a Service without running migrations. Use only from
// tests that drive the database via sqlmock.
func NewTestService(db database.Client, slackSvc slacksvc.Service) Service {
	return newServiceUnchecked(db, slackSvc)
}

func newServiceUnchecked(db database.Client, slackSvc slacksvc.Service) Service {
	return &service{db: db, slack: slackSvc}
}

func (s *service) Lock(ctx context.Context, cmd slack.SlashCommand) (*slack.Msg, error) {
	log := logger.WithContext(ctx).WithPrefix("[lock.Lock]")

	finderID, finderInlineName, ok := parseMention(cmd.Text)
	if !ok {
		return ephemeral("Usage: `/lock @someone` (your Slack app must have *Escape channels, users, and links* enabled)."), nil
	}

	victimID := cmd.UserID
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
	victimName := cmd.UserName
	if victimName == "" {
		v, err := s.slack.GetUser(ctx, victimID)
		if err != nil {
			return nil, fmt.Errorf("lookup victim: %w", err)
		}
		victimName = v.Name
	}

	if recent, err := s.hasRecentEvent(ctx, victimID, finderID); err != nil {
		return nil, err
	} else if recent {
		return ephemeral("You just locked this person. Give it a moment."), nil
	}

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

func (s *service) hasRecentEvent(ctx context.Context, victimID, finderID string) (bool, error) {
	q, args, err := s.db.Builder().
		From("lock_events").
		Select(goqu.L("1")).
		Where(goqu.Ex{
			"victim_id": victimID,
			"finder_id": finderID,
		}).
		Where(goqu.C("created_at").Gte(time.Now().Add(-cooldownWindow))).
		Prepared(true).
		ToSQL()
	if err != nil {
		return false, fmt.Errorf("build cooldown query: %w", err)
	}
	var x int
	switch err := s.db.DB().QueryRowContext(ctx, q, args...).Scan(&x); {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("cooldown check: %w", err)
	}
}

func (s *service) applyLock(ctx context.Context, victimID, victimName, finderID, finderName string) (victimPts, finderPts int, err error) {
	tx, err := s.db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.upsertPoints(ctx, tx, victimID, victimName, -1); err != nil {
		return 0, 0, fmt.Errorf("upsert victim: %w", err)
	}
	if err := s.upsertPoints(ctx, tx, finderID, finderName, 1); err != nil {
		return 0, 0, fmt.Errorf("upsert finder: %w", err)
	}
	if err := s.insertEvent(ctx, tx, victimID, finderID); err != nil {
		return 0, 0, fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit: %w", err)
	}

	// Post-commit reads; goqu lets us write one expression for both drivers.
	if victimPts, err = s.readPoints(ctx, victimID); err != nil {
		return 0, 0, fmt.Errorf("read victim points: %w", err)
	}
	if finderPts, err = s.readPoints(ctx, finderID); err != nil {
		return 0, 0, fmt.Errorf("read finder points: %w", err)
	}
	return victimPts, finderPts, nil
}

func (s *service) upsertPoints(ctx context.Context, tx *sql.Tx, userID, name string, delta int) error {
	q, args, err := s.db.Builder().
		Insert("lock_users").
		Prepared(true).
		Rows(goqu.Record{
			"slack_user_id":   userID,
			"slack_user_name": name,
			"points":          delta,
			"updated_at":      goqu.L("NOW()"),
		}).
		OnConflict(goqu.DoUpdate("slack_user_id", goqu.Record{
			"slack_user_name": name,
			// `points = points + delta` — portable across postgres + mysql.
			// Avoids EXCLUDED.points (postgres-only) and VALUES(points) (mysql-only).
			"points":     goqu.L("? + ?", goqu.C("points"), delta),
			"updated_at": goqu.L("NOW()"),
		})).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build upsert: %w", err)
	}
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return err
	}
	return nil
}

func (s *service) insertEvent(ctx context.Context, tx *sql.Tx, victimID, finderID string) error {
	q, args, err := s.db.Builder().
		Insert("lock_events").
		Prepared(true).
		Rows(goqu.Record{
			"victim_id":  victimID,
			"finder_id":  finderID,
			"created_at": goqu.L("NOW()"),
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build insert event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return err
	}
	return nil
}

func (s *service) readPoints(ctx context.Context, userID string) (int, error) {
	q, args, err := s.db.Builder().
		From("lock_users").
		Select("points").
		Where(goqu.Ex{"slack_user_id": userID}).
		Prepared(true).
		ToSQL()
	if err != nil {
		return 0, fmt.Errorf("build points query: %w", err)
	}
	var pts int
	if err := s.db.DB().QueryRowContext(ctx, q, args...).Scan(&pts); err != nil {
		return 0, err
	}
	return pts, nil
}

func (s *service) Leaderboard(ctx context.Context) (*slack.Msg, error) {
	top, err := s.queryLeaderboard(ctx, goqu.C("points").Desc())
	if err != nil {
		return nil, err
	}
	bottom, err := s.queryLeaderboard(ctx, goqu.C("points").Asc())
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

func (s *service) queryLeaderboard(ctx context.Context, order exp.OrderedExpression) ([]leaderboardRow, error) {
	q, args, err := s.db.Builder().
		From("lock_users").
		Select("slack_user_id", "slack_user_name", "points").
		Order(order).
		Limit(3).
		Prepared(true).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build leaderboard query: %w", err)
	}
	rows, err := s.db.DB().QueryContext(ctx, q, args...)
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

func ephemeral(text string) *slack.Msg {
	return &slack.Msg{ResponseType: slack.ResponseTypeEphemeral, Text: text}
}
