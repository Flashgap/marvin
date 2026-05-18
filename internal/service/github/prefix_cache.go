package github

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Flashgap/marvin/pkg/logger"
)

// PrefixFetcher fetches the current list of Linear team prefixes.
type PrefixFetcher func(ctx context.Context) ([]string, error)

type prefixSnapshot struct {
	prefixes     []string
	titleIssueID *regexp.Regexp
	branchID     *regexp.Regexp
	cleanTitle   *regexp.Regexp
	linearLink   *regexp.Regexp
}

// PrefixCache holds the current set of Linear team prefixes and their compiled regexes.
// Reads are lock-free; the snapshot is swapped atomically on refresh.
type PrefixCache struct {
	workspaceSlug string
	fetcher       PrefixFetcher
	snapshot      atomic.Pointer[prefixSnapshot]
}

// NewStaticPrefixCache returns a cache that never refreshes. Useful for tests and as a fallback.
func NewStaticPrefixCache(prefixes []string, workspaceSlug string) *PrefixCache {
	c := &PrefixCache{workspaceSlug: workspaceSlug}
	c.snapshot.Store(buildPrefixSnapshot(prefixes, workspaceSlug))
	return c
}

// NewPrefixCache builds a cache seeded with the given prefixes and refreshed via fetcher.
// Call Start to begin the background refresh loop.
func NewPrefixCache(workspaceSlug string, seed []string, fetcher PrefixFetcher) *PrefixCache {
	c := &PrefixCache{workspaceSlug: workspaceSlug, fetcher: fetcher}
	c.snapshot.Store(buildPrefixSnapshot(seed, workspaceSlug))
	return c
}

func buildPrefixSnapshot(prefixes []string, workspaceSlug string) *prefixSnapshot {
	quoted := make([]string, len(prefixes))
	for i, p := range prefixes {
		quoted[i] = regexp.QuoteMeta(p)
	}
	alt := strings.Join(quoted, "|")
	return &prefixSnapshot{
		prefixes:     prefixes,
		titleIssueID: regexp.MustCompile(fmt.Sprintf(`(?i)^(?:%s)(?:\s|-)\d+`, alt)),
		branchID:     regexp.MustCompile(fmt.Sprintf(`(?i)(?:%s)(?:\s|-)\d+`, alt)),
		cleanTitle:   regexp.MustCompile(fmt.Sprintf(`(?i)(?:(\w+)\s*\/(?:%s)\s*\d+\s*)?`, alt)),
		linearLink: regexp.MustCompile(fmt.Sprintf(
			`(?i)https:\/\/linear\.app\/%s\/issue\/((%s)-\d+)`,
			regexp.QuoteMeta(workspaceSlug),
			alt,
		)),
	}
}

func (c *PrefixCache) load() *prefixSnapshot {
	return c.snapshot.Load()
}

// WorkspaceSlug returns the Linear workspace slug the cache was built against.
func (c *PrefixCache) WorkspaceSlug() string {
	return c.workspaceSlug
}

func (c *PrefixCache) refresh(ctx context.Context) error {
	if c.fetcher == nil {
		return nil
	}
	prefixes, err := c.fetcher(ctx)
	if err != nil {
		return err
	}
	c.snapshot.Store(buildPrefixSnapshot(prefixes, c.workspaceSlug))
	return nil
}

// Start performs an initial fetch (keeping the seed if it fails) and then refreshes the
// cached prefixes every interval until ctx is cancelled. If interval is zero or fetcher is nil,
// no refresh loop is started and the seed values are used as-is.
func (c *PrefixCache) Start(ctx context.Context, interval time.Duration) {
	log := logger.WithContext(ctx).WithPrefix("[linear.PrefixCache]")
	if c.fetcher == nil || interval <= 0 {
		log.Infof("refresh disabled; using static prefixes (%d entries)", len(c.load().prefixes))
		return
	}

	if err := c.refresh(ctx); err != nil {
		log.Warnf("initial Linear prefix fetch failed, keeping seed (%d prefixes): %v", len(c.load().prefixes), err)
	} else {
		log.Infof("loaded %d Linear team prefixes", len(c.load().prefixes))
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping Linear prefix refresh loop")
				return
			case <-ticker.C:
				if err := c.refresh(ctx); err != nil {
					log.Warnf("Linear prefix refresh failed, keeping previous: %v", err)
				} else {
					log.Infof("refreshed Linear team prefixes: %d total", len(c.load().prefixes))
				}
			}
		}
	}()
}
