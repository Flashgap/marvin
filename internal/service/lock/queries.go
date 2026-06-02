package lock

import (
	"fmt"

	"github.com/Flashgap/marvin/pkg/database"
)

// queries holds the driver-specific SQL strings used by the lock service.
// All driver branching collapses to a single construction step at startup.
type queries struct {
	upsertUser     string // args: user_id, name, delta
	insertEvent    string // args: victim, finder
	recentEvent    string // args: victim, finder, cutoff
	topUsers       string // SELECT … ORDER BY points DESC LIMIT 3
	bottomUsers    string // SELECT … ORDER BY points ASC  LIMIT 3
}

func buildQueries(d database.Dialect) queries {
	// UPSERT on lock_users — INSERT or update the name and add `delta` to points.
	// The first call for a user inserts a row with points = delta and the given name.
	// Subsequent calls update name and adjust points.
	//
	// We can't use the generic Dialect.Upsert helper here because the update path
	// needs `points = points + delta` rather than `points = EXCLUDED.points`. So we
	// inline the dialect quirks for this one query.
	upsertUser := fmt.Sprintf(
		"INSERT INTO lock_users (slack_user_id, slack_user_name, points, updated_at) "+
			"VALUES (%s, %s, %s, NOW()) ",
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3),
	)
	// Detect dialect by the placeholder syntax it produces.
	if d.Placeholder(1) == "?" {
		upsertUser += "ON DUPLICATE KEY UPDATE " +
			"slack_user_name = VALUES(slack_user_name), " +
			"points = points + VALUES(points), " +
			"updated_at = NOW()"
	} else {
		upsertUser += "ON CONFLICT (slack_user_id) DO UPDATE SET " +
			"slack_user_name = EXCLUDED.slack_user_name, " +
			"points = lock_users.points + EXCLUDED.points, " +
			"updated_at = NOW()"
	}

	insertEvent := fmt.Sprintf(
		"INSERT INTO lock_events (victim_id, finder_id, created_at) VALUES (%s, %s, NOW())",
		d.Placeholder(1), d.Placeholder(2),
	)

	recentEvent := fmt.Sprintf(
		"SELECT 1 FROM lock_events WHERE victim_id = %s AND finder_id = %s AND created_at >= %s",
		d.Placeholder(1), d.Placeholder(2), d.Placeholder(3),
	)

	const leaderboardSelect = "SELECT slack_user_id, slack_user_name, points FROM lock_users ORDER BY points "
	topUsers := leaderboardSelect + "DESC LIMIT 3"
	bottomUsers := leaderboardSelect + "ASC LIMIT 3"

	return queries{
		upsertUser:  upsertUser,
		insertEvent: insertEvent,
		recentEvent: recentEvent,
		topUsers:    topUsers,
		bottomUsers: bottomUsers,
	}
}
