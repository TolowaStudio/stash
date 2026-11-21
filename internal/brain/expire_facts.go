package brain

import (
	"context"
	"fmt"
)

// ExpireFactsResult describes matching and expiry updates for admin expire-facts.
type ExpireFactsResult struct {
	Namespace string `json:"namespace"`
	Match     string `json:"match,omitempty"`
	Matched   int    `json:"matched"`
	Expired   int    `json:"expired"`
	DryRun    bool   `json:"dry_run"`
}

// ExpireFacts sets valid_until to now() on active facts in namespace, optionally filtered by content substring.
// Rows are never deleted; dryRun only counts matches.
func (b *Brain) ExpireFacts(ctx context.Context, namespaceSlug, contentSubstring string, dryRun bool) (ExpireFactsResult, error) {
	if err := validatePath(namespaceSlug); err != nil {
		return ExpireFactsResult{}, err
	}
	if namespaceSlug == "/" {
		return ExpireFactsResult{}, fmt.Errorf("brain: expire-facts requires a specific namespace, not /")
	}

	nsID, err := b.resolveNamespaceID(ctx, namespaceSlug)
	if err != nil {
		return ExpireFactsResult{}, err
	}

	result := ExpireFactsResult{
		Namespace: namespaceSlug,
		Match:     contentSubstring,
		DryRun:    dryRun,
	}

	var matched int
	countSQL := `SELECT COUNT(*) FROM facts
		WHERE namespace_id = $1 AND deleted_at IS NULL AND valid_until IS NULL`
	countArgs := []any{nsID}
	if contentSubstring != "" {
		countSQL += ` AND content ILIKE '%' || $2 || '%'`
		countArgs = append(countArgs, contentSubstring)
	}
	if err := b.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&matched); err != nil {
		return result, fmt.Errorf("count facts: %w", err)
	}
	result.Matched = matched

	if dryRun || matched == 0 {
		return result, nil
	}

	updateSQL := `UPDATE facts SET valid_until = now(), updated_at = now()
		WHERE namespace_id = $1 AND deleted_at IS NULL AND valid_until IS NULL`
	updateArgs := []any{nsID}
	if contentSubstring != "" {
		updateSQL += ` AND content ILIKE '%' || $2 || '%'`
		updateArgs = append(updateArgs, contentSubstring)
	}
	tag, err := b.pool.Exec(ctx, updateSQL, updateArgs...)
	if err != nil {
		return result, fmt.Errorf("expire facts: %w", err)
	}
	result.Expired = int(tag.RowsAffected())
	return result, nil
}