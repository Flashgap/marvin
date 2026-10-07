package standup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"time"

	"github.com/doug-martin/goqu/v9"

	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	"github.com/Flashgap/marvin/pkg/database"
	"github.com/Flashgap/marvin/pkg/logger"
	pkgslack "github.com/Flashgap/marvin/pkg/slack"
)

// historyLookback is how far back Remind reads the channel to find each
// member's last update. Members who haven't posted within it keep their
// stored one.
const historyLookback = 14 * 24 * time.Hour

type service struct {
	db        database.Client
	slack     slacksvc.Service
	channelID string
	now       func() time.Time
}

// userState is what the database holds for one member: their stored last
// update and whether they were already reminded today.
type userState struct {
	lastTS        string
	lastText      string
	remindedToday bool
}

// NewService applies pending migrations and wires the standup service. A
// non-nil db is required; pass migrations via mfs (typically
// internal/migrations.FS).
func NewService(ctx context.Context, db database.Client, slackSvc slacksvc.Service, channelID string, mfs fs.FS) (Service, error) {
	if db == nil {
		return nil, fmt.Errorf("standup: database client is required")
	}
	if slackSvc == nil {
		return nil, fmt.Errorf("standup: slack service is required")
	}
	if channelID == "" {
		return nil, fmt.Errorf("standup: channel ID is required")
	}
	if err := db.Migrate(ctx, mfs); err != nil {
		return nil, fmt.Errorf("standup: applying migrations: %w", err)
	}
	return newServiceUnchecked(db, slackSvc, channelID, time.Now), nil
}

// NewTestService builds a Service without running migrations, with now as its
// clock. Use only from tests that drive the database via sqlmock.
func NewTestService(db database.Client, slackSvc slacksvc.Service, channelID string, now func() time.Time) Service {
	return newServiceUnchecked(db, slackSvc, channelID, now)
}

func newServiceUnchecked(db database.Client, slackSvc slacksvc.Service, channelID string, now func() time.Time) Service {
	return &service{db: db, slack: slackSvc, channelID: channelID, now: now}
}

func (s *service) Remind(ctx context.Context) (*Report, error) {
	log := logger.WithContext(ctx).WithPrefix("[standup.Remind]")

	now := s.now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	members, err := s.slack.ChannelMembers(ctx, s.channelID)
	if err != nil {
		return nil, fmt.Errorf("list channel members: %w", err)
	}
	history, err := s.slack.ChannelHistory(ctx, s.channelID, now.Add(-historyLookback))
	if err != nil {
		return nil, fmt.Errorf("read channel history: %w", err)
	}
	latest := latestUpdates(history)
	states, err := s.readStates(ctx, todayStart)
	if err != nil {
		return nil, err
	}

	report := &Report{Members: len(members)}
	// Every member is processed even when one fails; the joined error makes
	// the caller retry, and the retry skips whoever was already reminded.
	var errs []error
	for _, userID := range members {
		state := states[userID]
		if state.remindedToday {
			report.AlreadyReminded++
			continue
		}

		if m, ok := latest[userID]; ok {
			// A failed write must not block the DM decision: log it and go on.
			if m.TS != state.lastTS || m.Text != state.lastText {
				if err := s.saveLastUpdate(ctx, userID, m, now); err != nil {
					log.Warnf("failed to store the last update of %s: %v", userID, err)
					errs = append(errs, fmt.Errorf("store last update of %s: %w", userID, err))
				}
			}
			state.lastTS, state.lastText = m.TS, m.Text
			if !m.PostedAt.Before(todayStart) {
				report.AlreadyPosted++
				continue
			}
		}

		user, err := s.slack.GetUser(ctx, userID)
		if err != nil {
			log.Warnf("failed to look up %s: %v", userID, err)
			errs = append(errs, fmt.Errorf("lookup %s: %w", userID, err))
			continue
		}
		if user.IsBot || user.Deleted {
			report.Ignored++
			continue
		}

		err = s.slack.SendDM(ctx, userID, formatReminder(s.channelID, state.lastTS, state.lastText))
		if pkgslack.IsPermanentDMError(err) {
			// Not marked reminded: tomorrow's run tries again, harmlessly.
			log.Warnf("cannot DM %s, ignoring: %v", userID, err)
			report.Ignored++
			continue
		}
		if err != nil {
			log.Warnf("failed to DM %s: %v", userID, err)
			errs = append(errs, fmt.Errorf("DM %s: %w", userID, err))
			continue
		}

		if err := s.markReminded(ctx, userID, now); err != nil {
			log.Warnf("failed to mark %s as reminded: %v", userID, err)
			errs = append(errs, fmt.Errorf("mark %s as reminded: %w", userID, err))
			continue
		}
		report.Reminded++
	}

	log.Infof("%d members: %d reminded, %d already posted, %d already reminded, %d ignored, %d failed",
		report.Members, report.Reminded, report.AlreadyPosted, report.AlreadyReminded, report.Ignored, len(errs))
	return report, errors.Join(errs...)
}

// readStates loads every stored member. Timestamps are only compared in SQL,
// never scanned, because the MySQL DSN doesn't enable parseTime.
func (s *service) readStates(ctx context.Context, todayStart time.Time) (map[string]userState, error) {
	q, args, err := s.db.Builder().
		From("standup_users").
		Select("slack_user_id", "last_update_ts", "last_update_text",
			// CASE yields an integer on both drivers, where a bare comparison
			// scans as a bool on Postgres and an int on MySQL.
			goqu.L("CASE WHEN last_reminded_at >= ? THEN 1 ELSE 0 END", todayStart)).
		Prepared(true).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build standup users query: %w", err)
	}
	rows, err := s.db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("standup users query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	states := make(map[string]userState)
	for rows.Next() {
		var (
			userID           string
			lastTS, lastText sql.NullString
			remindedToday    int
		)
		if err := rows.Scan(&userID, &lastTS, &lastText, &remindedToday); err != nil {
			return nil, fmt.Errorf("standup users scan: %w", err)
		}
		states[userID] = userState{lastTS: lastTS.String, lastText: lastText.String, remindedToday: remindedToday == 1}
	}
	return states, rows.Err()
}

func (s *service) saveLastUpdate(ctx context.Context, userID string, m slacksvc.Message, now time.Time) error {
	return s.upsertUser(ctx, userID, goqu.Record{
		"last_update_ts":   m.TS,
		"last_update_text": m.Text,
		"updated_at":       now,
	})
}

func (s *service) markReminded(ctx context.Context, userID string, now time.Time) error {
	return s.upsertUser(ctx, userID, goqu.Record{
		"last_reminded_at": now,
		"updated_at":       now,
	})
}

// upsertUser sets fields on the member's row, creating it if needed.
// Timestamps are written from Go in UTC, never with NOW(), whose result
// depends on the session timezone.
func (s *service) upsertUser(ctx context.Context, userID string, fields goqu.Record) error {
	row := goqu.Record{"slack_user_id": userID}
	maps.Copy(row, fields)
	q, args, err := s.db.Builder().
		Insert("standup_users").
		Prepared(true).
		Rows(row).
		OnConflict(goqu.DoUpdate("slack_user_id", fields)).
		ToSQL()
	if err != nil {
		return fmt.Errorf("build upsert: %w", err)
	}
	if _, err := s.db.DB().ExecContext(ctx, q, args...); err != nil {
		return err
	}
	return nil
}
