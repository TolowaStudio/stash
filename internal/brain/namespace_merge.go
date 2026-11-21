package brain

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// NamespaceMergeCounts holds row counts that would move or did move during a merge.
type NamespaceMergeCounts struct {
	Episodes      int64 `json:"episodes"`
	Facts         int64 `json:"facts"`
	Relationships int64 `json:"relationships"`
	CausalLinks   int64 `json:"causal_links"`
	Contexts      int64 `json:"contexts"`
	Goals         int64 `json:"goals"`
	Patterns      int64 `json:"patterns"`
	Hypotheses    int64 `json:"hypotheses"`
	Failures      int64 `json:"failures"`
	Contradictions int64 `json:"contradictions"`
	Quarantine    int64 `json:"consolidation_quarantine"`
	MemoryUtility int64 `json:"memory_utility"`
}

// NamespaceMergeResult is returned from MergeNamespace.
type NamespaceMergeResult struct {
	FromSlug  string               `json:"from_slug"`
	IntoSlug  string               `json:"into_slug"`
	DryRun    bool                 `json:"dry_run"`
	Counts    NamespaceMergeCounts   `json:"counts"`
	MergeID   *int64               `json:"merge_id,omitempty"`
}

var (
	ErrNamespaceMergeSame      = fmt.Errorf("brain: cannot merge a namespace into itself")
	ErrNamespaceAlreadyMerged  = fmt.Errorf("brain: source namespace is already merged")
	ErrNamespaceMergeTarget    = fmt.Errorf("brain: target namespace is merged into another namespace")
)

func (b *Brain) countNamespaceRows(ctx context.Context, nsID int64) (NamespaceMergeCounts, error) {
	var c NamespaceMergeCounts
	tables := []struct {
		name  string
		query string
		dest  *int64
	}{
		{"episodes", "SELECT COUNT(*) FROM episodes WHERE namespace_id = $1", &c.Episodes},
		{"facts", "SELECT COUNT(*) FROM facts WHERE namespace_id = $1", &c.Facts},
		{"relationships", "SELECT COUNT(*) FROM relationships WHERE namespace_id = $1", &c.Relationships},
		{"causal_links", "SELECT COUNT(*) FROM causal_links WHERE namespace_id = $1", &c.CausalLinks},
		{"contexts", "SELECT COUNT(*) FROM contexts WHERE namespace_id = $1", &c.Contexts},
		{"goals", "SELECT COUNT(*) FROM goals WHERE namespace_id = $1", &c.Goals},
		{"patterns", "SELECT COUNT(*) FROM patterns WHERE namespace_id = $1", &c.Patterns},
		{"hypotheses", "SELECT COUNT(*) FROM hypotheses WHERE namespace_id = $1", &c.Hypotheses},
		{"failures", "SELECT COUNT(*) FROM failures WHERE namespace_id = $1", &c.Failures},
		{"contradictions", "SELECT COUNT(*) FROM contradictions WHERE namespace_id = $1", &c.Contradictions},
		{"consolidation_quarantine", "SELECT COUNT(*) FROM consolidation_quarantine WHERE namespace_id = $1", &c.Quarantine},
		{"memory_utility", "SELECT COUNT(*) FROM memory_utility WHERE namespace_id = $1", &c.MemoryUtility},
	}
	for _, t := range tables {
		if err := b.pool.QueryRow(ctx, t.query, nsID).Scan(t.dest); err != nil {
			return c, fmt.Errorf("count %s: %w", t.name, err)
		}
	}
	return c, nil
}

// MergeNamespace moves memory rows from --from into --into and marks from as merged.
// dryRun defaults to true at the CLI; when true, only counts are returned.
func (b *Brain) MergeNamespace(ctx context.Context, fromSlug, intoSlug string, dryRun bool) (*NamespaceMergeResult, error) {
	if fromSlug == intoSlug {
		return nil, ErrNamespaceMergeSame
	}
	if err := validatePath(fromSlug); err != nil {
		return nil, fmt.Errorf("from slug: %w", err)
	}
	if err := validatePath(intoSlug); err != nil {
		return nil, fmt.Errorf("into slug: %w", err)
	}

	fromID, fromMerged, err := b.lookupNamespaceMergeState(ctx, fromSlug)
	if err != nil {
		return nil, fmt.Errorf("from namespace: %w", err)
	}
	intoID, intoMerged, err := b.lookupNamespaceMergeState(ctx, intoSlug)
	if err != nil {
		return nil, fmt.Errorf("into namespace: %w", err)
	}
	if fromMerged != nil {
		return nil, ErrNamespaceAlreadyMerged
	}
	if intoMerged != nil {
		return nil, ErrNamespaceMergeTarget
	}

	counts, err := b.countNamespaceRows(ctx, fromID)
	if err != nil {
		return nil, err
	}

	result := &NamespaceMergeResult{
		FromSlug: fromSlug,
		IntoSlug: intoSlug,
		DryRun:   dryRun,
		Counts:   counts,
	}
	if dryRun {
		return result, nil
	}

	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin merge tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var mergeID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO namespace_merges (from_namespace_id, into_namespace_id) VALUES ($1, $2) RETURNING id`,
		fromID, intoID,
	).Scan(&mergeID)
	if err != nil {
		return nil, fmt.Errorf("create merge log: %w", err)
	}

	moves := []struct {
		table string
		sql   string
	}{
		{"episodes", "UPDATE episodes SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"facts", "UPDATE facts SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"relationships", "UPDATE relationships SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"causal_links", "UPDATE causal_links SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"goals", "UPDATE goals SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"patterns", "UPDATE patterns SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"hypotheses", "UPDATE hypotheses SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"failures", "UPDATE failures SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"contradictions", "UPDATE contradictions SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"consolidation_quarantine", "UPDATE consolidation_quarantine SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
		{"memory_utility", "UPDATE memory_utility SET namespace_id = $2 WHERE namespace_id = $1 RETURNING id"},
	}
	for _, m := range moves {
		if err := b.logAndMoveRows(ctx, tx, mergeID, m.table, m.sql, fromID, intoID); err != nil {
			return nil, err
		}
	}

	// Context row is keyed by namespace_id — drop source row if target already has one.
	var intoCtxExists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM contexts WHERE namespace_id = $1)", intoID).Scan(&intoCtxExists); err != nil {
		return nil, fmt.Errorf("check target context: %w", err)
	}
	if intoCtxExists {
		var deleted []int64
		rows, err := tx.Query(ctx, "DELETE FROM contexts WHERE namespace_id = $1 RETURNING namespace_id", fromID)
		if err != nil {
			return nil, fmt.Errorf("delete source context: %w", err)
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			deleted = append(deleted, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, id := range deleted {
			if _, err := tx.Exec(ctx,
				`INSERT INTO namespace_merge_moves (merge_id, table_name, row_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
				mergeID, "contexts", id); err != nil {
				return nil, fmt.Errorf("log context delete: %w", err)
			}
		}
	} else if err := b.logAndMoveRows(ctx, tx, mergeID, "contexts",
		"UPDATE contexts SET namespace_id = $2 WHERE namespace_id = $1 RETURNING namespace_id",
		fromID, intoID); err != nil {
		return nil, err
	}

	// Merge consolidation_progress checkpoints (max of each counter), then drop source row.
	if err := b.mergeConsolidationProgress(ctx, tx, mergeID, fromID, intoID); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE namespaces SET merged_into = $2, updated_at = now() WHERE id = $1`,
		fromID, intoID); err != nil {
		return nil, fmt.Errorf("mark namespace merged: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit merge: %w", err)
	}

	result.MergeID = &mergeID
	return result, nil
}

func (b *Brain) lookupNamespaceMergeState(ctx context.Context, slug string) (id int64, mergedInto *int64, err error) {
	err = b.pool.QueryRow(ctx,
		`SELECT id, merged_into FROM namespaces WHERE slug = $1`, slug,
	).Scan(&id, &mergedInto)
	if err == pgx.ErrNoRows {
		return 0, nil, ErrNamespaceNotFound
	}
	if err != nil {
		return 0, nil, fmt.Errorf("lookup namespace: %w", err)
	}
	return id, mergedInto, nil
}

func (b *Brain) logAndMoveRows(ctx context.Context, tx pgx.Tx, mergeID int64, table, updateSQL string, fromID, intoID int64) error {
	rows, err := tx.Query(ctx, updateSQL, fromID, intoID)
	if err != nil {
		return fmt.Errorf("move %s: %w", table, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan %s id: %w", table, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := tx.Exec(ctx,
			`INSERT INTO namespace_merge_moves (merge_id, table_name, row_id) VALUES ($1, $2, $3)`,
			mergeID, table, id); err != nil {
			return fmt.Errorf("log %s move: %w", table, err)
		}
	}
	return nil
}

func (b *Brain) mergeConsolidationProgress(ctx context.Context, tx pgx.Tx, mergeID, fromID, intoID int64) error {
	var fromExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM consolidation_progress WHERE namespace_id = $1)`, fromID,
	).Scan(&fromExists); err != nil {
		return err
	}
	if !fromExists {
		return nil
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO consolidation_progress (namespace_id)
		VALUES ($1)
		ON CONFLICT (namespace_id) DO NOTHING`, intoID)
	if err != nil {
		return fmt.Errorf("ensure target consolidation progress: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE consolidation_progress AS into_row SET
			last_episode_id = GREATEST(into_row.last_episode_id, from_row.last_episode_id),
			last_fact_id = GREATEST(into_row.last_fact_id, from_row.last_fact_id),
			last_relationship_id = GREATEST(into_row.last_relationship_id, from_row.last_relationship_id),
			last_pattern_fact_id = GREATEST(into_row.last_pattern_fact_id, from_row.last_pattern_fact_id),
			last_pattern_rel_id = GREATEST(into_row.last_pattern_rel_id, from_row.last_pattern_rel_id),
			last_goal_progress_fact_id = GREATEST(into_row.last_goal_progress_fact_id, from_row.last_goal_progress_fact_id),
			last_failure_id = GREATEST(into_row.last_failure_id, from_row.last_failure_id),
			last_failure_episode_id = GREATEST(into_row.last_failure_episode_id, from_row.last_failure_episode_id),
			last_hypothesis_fact_id = GREATEST(into_row.last_hypothesis_fact_id, from_row.last_hypothesis_fact_id),
			last_causal_fact_id = GREATEST(into_row.last_causal_fact_id, from_row.last_causal_fact_id),
			updated_at = now()
		FROM consolidation_progress AS from_row
		WHERE into_row.namespace_id = $1 AND from_row.namespace_id = $2`,
		intoID, fromID)
	if err != nil {
		return fmt.Errorf("merge consolidation progress: %w", err)
	}

	rows, err := tx.Query(ctx,
		`DELETE FROM consolidation_progress WHERE namespace_id = $1 RETURNING namespace_id`, fromID)
	if err != nil {
		return fmt.Errorf("delete source consolidation progress: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO namespace_merge_moves (merge_id, table_name, row_id) VALUES ($1, $2, $3)`,
			mergeID, "consolidation_progress", id); err != nil {
			return err
		}
	}
	return rows.Err()
}

// FindNamespaceByNormalizedSlug returns the canonical slug when normalized forms collide.
func (b *Brain) FindNamespaceByNormalizedSlug(ctx context.Context, slug string) (existingSlug string, existingID int64, found bool, err error) {
	normalized := NormalizeNamespaceSlug(slug)
	if normalized == "" {
		return "", 0, false, nil
	}
	err = b.pool.QueryRow(ctx,
		`SELECT slug, id FROM namespaces
		 WHERE normalized_slug = $1 AND merged_into IS NULL
		 ORDER BY created_at ASC LIMIT 1`,
		normalized,
	).Scan(&existingSlug, &existingID)
	if err == pgx.ErrNoRows {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, fmt.Errorf("find normalized namespace: %w", err)
	}
	if existingSlug == slug {
		return existingSlug, existingID, false, nil
	}
	return existingSlug, existingID, true, nil
}