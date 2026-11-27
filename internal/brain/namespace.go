package brain

import (
	"context"
	"fmt"
	"time"

	"github.com/alash3al/stash/internal/models"
	"github.com/jackc/pgx/v5"
)

// CreateNamespace creates a new namespace with the given slug, name, and description.
// Parent namespaces are auto-created with slug as name if they don't exist.
func (b *Brain) CreateNamespace(ctx context.Context, slug, name, description string) (int64, error) {
	if err := validatePath(slug); err != nil {
		return 0, err
	}

	segments := splitPath(slug)
	if len(segments) == 0 {
		var id int64
		err := b.pool.QueryRow(ctx,
			"INSERT INTO namespaces (slug, name) VALUES ('/', '/') ON CONFLICT (slug) DO UPDATE SET updated_at = now() RETURNING id",
		).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("create root namespace: %w", err)
		}
		return id, nil
	}

	currentPath := ""
	for i, seg := range segments {
		currentPath += "/" + seg
		if i < len(segments)-1 {
			var id int64
			_ = b.pool.QueryRow(ctx,
				"INSERT INTO namespaces (slug, name) VALUES ($1, $1) ON CONFLICT (slug) DO UPDATE SET updated_at = now() RETURNING id",
				currentPath,
			).Scan(&id)
		}
	}

	var id int64
	err := b.pool.QueryRow(ctx,
		"INSERT INTO namespaces (slug, name, description) VALUES ($1, $2, $3) ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, updated_at = now() RETURNING id",
		slug, name, description,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create namespace: %w", err)
	}
	return id, nil
}

// GetNamespace returns a namespace by slug.
func (b *Brain) GetNamespace(ctx context.Context, slug string) (*models.Namespace, error) {
	var ns models.Namespace
	err := b.pool.QueryRow(ctx,
		"SELECT id, slug, name, description, created_at, updated_at FROM namespaces WHERE slug = $1",
		slug,
	).Scan(&ns.ID, &ns.Slug, &ns.Name, &ns.Description, &ns.CreatedAt, &ns.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNamespaceNotFound
		}
		return nil, fmt.Errorf("get namespace: %w", err)
	}
	return &ns, nil
}

// ListNamespaces returns namespaces, optionally filtered by slug paths.
// If slugs is empty, returns all namespaces.
// Each path matches itself and all descendants.
func (b *Brain) ListNamespaces(ctx context.Context, slugs []string, page Pagination) ([]models.Namespace, error) {
	page = page.Sanitize()

	if len(slugs) == 0 {
		rows, err := b.pool.Query(ctx,
			"SELECT id, slug, name, description, created_at, updated_at FROM namespaces ORDER BY slug LIMIT $1 OFFSET $2",
			page.Limit, page.Offset,
		)
		if err != nil {
			return nil, fmt.Errorf("list namespaces: %w", err)
		}
		defer rows.Close()

		var result []models.Namespace
		for rows.Next() {
			var ns models.Namespace
			if err := rows.Scan(&ns.ID, &ns.Slug, &ns.Name, &ns.Description, &ns.CreatedAt, &ns.UpdatedAt); err != nil {
				return nil, fmt.Errorf("scan namespace: %w", err)
			}
			result = append(result, ns)
		}
		return result, rows.Err()
	}

	ids, err := b.resolveNamespaceIDs(ctx, slugs)
	if err != nil {
		return nil, err
	}

	rows, err := b.pool.Query(ctx,
		"SELECT id, slug, name, description, created_at, updated_at FROM namespaces WHERE id = ANY($1) ORDER BY slug LIMIT $2 OFFSET $3",
		ids, page.Limit, page.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	defer rows.Close()

	var result []models.Namespace
	for rows.Next() {
		var ns models.Namespace
		if err := rows.Scan(&ns.ID, &ns.Slug, &ns.Name, &ns.Description, &ns.CreatedAt, &ns.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan namespace: %w", err)
		}
		result = append(result, ns)
	}
	return result, rows.Err()
}

// GetOrCreateConsolidationProgress returns progress for a namespace, creating a row if needed.
func (b *Brain) GetOrCreateConsolidationProgress(ctx context.Context, namespaceID int64) (*models.ConsolidationProgress, error) {
	var cp models.ConsolidationProgress
	err := b.pool.QueryRow(ctx,
		`INSERT INTO consolidation_progress (namespace_id) VALUES ($1)
		 ON CONFLICT (namespace_id) DO UPDATE SET updated_at = consolidation_progress.updated_at
		 RETURNING namespace_id, last_episode_id, last_fact_id, last_relationship_id, last_pattern_fact_id, last_pattern_rel_id, last_goal_progress_fact_id, last_failure_id, last_failure_episode_id, last_hypothesis_fact_id, last_causal_fact_id, last_decay_run, last_run, updated_at`,
		namespaceID,
	).Scan(&cp.NamespaceID, &cp.LastEpisodeID, &cp.LastFactID, &cp.LastRelationshipID, &cp.LastPatternFactID, &cp.LastPatternRelID, &cp.LastGoalProgressFactID, &cp.LastFailureID, &cp.LastFailureEpisodeID, &cp.LastHypothesisFactID, &cp.LastCausalFactID, &cp.LastDecayRun, &cp.LastRun, &cp.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get consolidation progress: %w", err)
	}
	return &cp, nil
}

// SaveConsolidationProgress updates the checkpoint for a namespace.
func (b *Brain) SaveConsolidationProgress(ctx context.Context, cp models.ConsolidationProgress) error {
	now := time.Now().UTC()
	_, err := b.pool.Exec(ctx,
		`UPDATE consolidation_progress SET
			last_episode_id = $2, last_fact_id = $3, last_relationship_id = $4,
			last_pattern_fact_id = $5, last_pattern_rel_id = $6,
			last_goal_progress_fact_id = $7, last_failure_id = $8, last_failure_episode_id = $9, last_hypothesis_fact_id = $10,
			last_causal_fact_id = $11, last_decay_run = $12, last_run = $13, updated_at = $14
		 WHERE namespace_id = $1`,
		cp.NamespaceID, cp.LastEpisodeID, cp.LastFactID, cp.LastRelationshipID,
		cp.LastPatternFactID, cp.LastPatternRelID,
		cp.LastGoalProgressFactID, cp.LastFailureID, cp.LastFailureEpisodeID, cp.LastHypothesisFactID,
		cp.LastCausalFactID, cp.LastDecayRun, now, now,
	)
	if err != nil {
		return fmt.Errorf("save consolidation progress: %w", err)
	}
	return nil
}

// UpdateNamespaceDescription replaces the description of an existing namespace.
// Returns ErrNamespaceNotFound if the slug does not exist.
func (b *Brain) UpdateNamespaceDescription(ctx context.Context, slug, description string) error {
	tag, err := b.pool.Exec(ctx,
		"UPDATE namespaces SET description = $2, updated_at = now() WHERE slug = $1",
		slug, description,
	)
	if err != nil {
		return fmt.Errorf("update namespace description: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNamespaceNotFound
	}
	return nil
}

// RenameNamespace changes the slug (and name, when it still equals the old slug) of a
// namespace. It refuses to rename when the namespace has descendants or the target slug
// is already taken, so no memories or hierarchy are rewritten implicitly.
func (b *Brain) RenameNamespace(ctx context.Context, from, to string) error {
	if err := validatePath(to); err != nil {
		return err
	}
	if to == "" || to == "/" {
		return fmt.Errorf("rename namespace: target slug must not be root")
	}
	if from == to {
		return fmt.Errorf("rename namespace: source and target are the same")
	}
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("rename namespace: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var fromID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM namespaces WHERE slug = $1 FOR UPDATE", from).Scan(&fromID); err != nil {
		if err == pgx.ErrNoRows {
			return ErrNamespaceNotFound
		}
		return fmt.Errorf("rename namespace: lookup source: %w", err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM namespaces WHERE slug = $1)", to).Scan(&exists); err != nil {
		return fmt.Errorf("rename namespace: lookup target: %w", err)
	}
	if exists {
		return fmt.Errorf("rename namespace: target %q already exists; use merge", to)
	}
	var children int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM namespaces WHERE slug LIKE $1", likePatternForDescendants(from)).Scan(&children); err != nil {
		return fmt.Errorf("rename namespace: count descendants: %w", err)
	}
	if children > 0 {
		return fmt.Errorf("rename namespace: %q has %d descendant namespaces; rename or merge them first", from, children)
	}
	if _, err := tx.Exec(ctx,
		"UPDATE namespaces SET slug = $2, name = CASE WHEN name = slug THEN $2 ELSE name END, updated_at = now() WHERE id = $1",
		fromID, to,
	); err != nil {
		return fmt.Errorf("rename namespace: update: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("rename namespace: commit: %w", err)
	}
	return nil
}

// MemoryCounts is the number of rows per memory table that belong to one namespace.
type MemoryCounts struct {
	Episodes       int64 `json:"episodes"`
	Facts          int64 `json:"facts"`
	Relationships  int64 `json:"relationships"`
	Patterns       int64 `json:"patterns"`
	CausalLinks    int64 `json:"causal_links"`
	Goals          int64 `json:"goals"`
	Hypotheses     int64 `json:"hypotheses"`
	Failures       int64 `json:"failures"`
	Contradictions int64 `json:"contradictions"`
}

// Total is the sum of all memory rows.
func (c MemoryCounts) Total() int64 {
	return c.Episodes + c.Facts + c.Relationships + c.Patterns + c.CausalLinks +
		c.Goals + c.Hypotheses + c.Failures + c.Contradictions
}

// memoryTables lists every table whose rows carry a namespace_id that a merge moves.
var memoryTables = []struct {
	name  string
	field func(*MemoryCounts) *int64
}{
	{"episodes", func(c *MemoryCounts) *int64 { return &c.Episodes }},
	{"facts", func(c *MemoryCounts) *int64 { return &c.Facts }},
	{"relationships", func(c *MemoryCounts) *int64 { return &c.Relationships }},
	{"patterns", func(c *MemoryCounts) *int64 { return &c.Patterns }},
	{"causal_links", func(c *MemoryCounts) *int64 { return &c.CausalLinks }},
	{"goals", func(c *MemoryCounts) *int64 { return &c.Goals }},
	{"hypotheses", func(c *MemoryCounts) *int64 { return &c.Hypotheses }},
	{"failures", func(c *MemoryCounts) *int64 { return &c.Failures }},
	{"contradictions", func(c *MemoryCounts) *int64 { return &c.Contradictions }},
}

// CountNamespaceMemories returns per-table memory counts for one namespace id.
func (b *Brain) CountNamespaceMemories(ctx context.Context, namespaceID int64) (MemoryCounts, error) {
	return countMemories(ctx, b.pool, namespaceID)
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func countMemories(ctx context.Context, q querier, namespaceID int64) (MemoryCounts, error) {
	var c MemoryCounts
	for _, t := range memoryTables {
		if err := q.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s WHERE namespace_id = $1", t.name), namespaceID).Scan(t.field(&c)); err != nil {
			return c, fmt.Errorf("count %s: %w", t.name, err)
		}
	}
	return c, nil
}

// MergeResult reports what MergeNamespaces moved.
type MergeResult struct {
	Source MemoryCounts `json:"source_before"`
	Target MemoryCounts `json:"target_before"`
	Final  MemoryCounts `json:"target_after"`
}

// MergeNamespaces moves every memory row from source into target and removes the
// source namespace row. No memory is deleted: the target's post-merge counts equal
// source-before plus target-before for every table. Per-namespace checkpoints
// (contexts, consolidation_progress) are kept on the target when it has one and
// otherwise moved; the source's leftovers are dropped. Source must have no descendants.
func (b *Brain) MergeNamespaces(ctx context.Context, source, target string) (MergeResult, error) {
	var res MergeResult
	if source == target {
		return res, fmt.Errorf("merge namespaces: source and target are the same")
	}
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("merge namespaces: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var srcID, tgtID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM namespaces WHERE slug = $1 FOR UPDATE", source).Scan(&srcID); err != nil {
		if err == pgx.ErrNoRows {
			return res, ErrNamespaceNotFound
		}
		return res, fmt.Errorf("merge namespaces: lookup source: %w", err)
	}
	if err := tx.QueryRow(ctx, "SELECT id FROM namespaces WHERE slug = $1 FOR UPDATE", target).Scan(&tgtID); err != nil {
		if err == pgx.ErrNoRows {
			return res, ErrNamespaceNotFound
		}
		return res, fmt.Errorf("merge namespaces: lookup target: %w", err)
	}
	var children int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM namespaces WHERE slug LIKE $1", likePatternForDescendants(source)).Scan(&children); err != nil {
		return res, fmt.Errorf("merge namespaces: count descendants: %w", err)
	}
	if children > 0 {
		return res, fmt.Errorf("merge namespaces: %q has %d descendant namespaces; merge them first", source, children)
	}

	if res.Source, err = countMemories(ctx, tx, srcID); err != nil {
		return res, err
	}
	if res.Target, err = countMemories(ctx, tx, tgtID); err != nil {
		return res, err
	}

	for _, t := range memoryTables {
		if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s SET namespace_id = $2 WHERE namespace_id = $1", t.name), srcID, tgtID); err != nil {
			return res, fmt.Errorf("merge namespaces: move %s: %w", t.name, err)
		}
	}

	for _, table := range []string{"contexts", "consolidation_progress"} {
		if _, err := tx.Exec(ctx, fmt.Sprintf(
			"UPDATE %[1]s SET namespace_id = $2 WHERE namespace_id = $1 AND NOT EXISTS (SELECT 1 FROM %[1]s WHERE namespace_id = $2)", table),
			srcID, tgtID); err != nil {
			return res, fmt.Errorf("merge namespaces: move %s: %w", table, err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE namespace_id = $1", table), srcID); err != nil {
			return res, fmt.Errorf("merge namespaces: drop %s checkpoint: %w", table, err)
		}
	}

	if _, err := tx.Exec(ctx, "DELETE FROM namespaces WHERE id = $1", srcID); err != nil {
		return res, fmt.Errorf("merge namespaces: delete source: %w", err)
	}
	if _, err := tx.Exec(ctx, "UPDATE namespaces SET updated_at = now() WHERE id = $1", tgtID); err != nil {
		return res, fmt.Errorf("merge namespaces: touch target: %w", err)
	}

	if res.Final, err = countMemories(ctx, tx, tgtID); err != nil {
		return res, err
	}
	if res.Final.Total() != res.Source.Total()+res.Target.Total() {
		return res, fmt.Errorf("merge namespaces: count mismatch: target after %d != source %d + target %d",
			res.Final.Total(), res.Source.Total(), res.Target.Total())
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("merge namespaces: commit: %w", err)
	}
	return res, nil
}
