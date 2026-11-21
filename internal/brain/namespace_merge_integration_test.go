package brain_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alash3al/stash/internal/brain"
	"github.com/alash3al/stash/internal/db"
	"github.com/alash3al/stash/internal/queries"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNamespaceMergeAndCollisionFailsWithoutFix(t *testing.T) {
	dsn := os.Getenv("STASH_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("STASH_TEST_POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, dsn, "test-embedding", 3)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	q, err := queries.New()
	if err != nil {
		t.Fatal(err)
	}
	br, err := brain.New(pool, fixedEmbedder{vector: []float32{1, 0, 0}}, noopReasoner{}, q, brain.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()

	prefix := fmt.Sprintf("/ns-merge-%d", time.Now().UnixNano())
	fromSlug := prefix + "/motion-control"
	intoSlug := prefix + "/motion/control"

	if _, err := createNamespaceRow(ctx, pool, fromSlug); err != nil {
		t.Fatal(err)
	}
	fromID, err := br.ResolveNamespaceIDs(ctx, []string{fromSlug})
	if err != nil || len(fromID) != 1 {
		t.Fatalf("from namespace: %v %v", fromID, err)
	}
	if _, err := createNamespaceRow(ctx, pool, intoSlug); err != nil {
		t.Fatal(err)
	}

	idBefore, err := br.Remember(ctx, fromSlug, "episode lives on twin A", nil)
	if err != nil {
		t.Fatal(err)
	}

	dry, err := br.MergeNamespace(ctx, fromSlug, intoSlug, true)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Counts.Episodes != 1 {
		t.Fatalf("dry-run episodes = %d, want 1", dry.Counts.Episodes)
	}

	applied, err := br.MergeNamespace(ctx, fromSlug, intoSlug, false)
	if err != nil {
		t.Fatal(err)
	}
	if applied.MergeID == nil {
		t.Fatal("expected merge id")
	}

	var mergedInto *int64
	if err := pool.QueryRow(ctx, `SELECT merged_into FROM namespaces WHERE slug = $1`, fromSlug).Scan(&mergedInto); err != nil {
		t.Fatal(err)
	}
	if mergedInto == nil {
		t.Fatal("from namespace should be marked merged")
	}

	idAfter, err := br.Remember(ctx, fromSlug, "write through merged alias", nil)
	if err != nil {
		t.Fatal(err)
	}
	intoIDs, err := br.ResolveNamespaceIDs(ctx, []string{intoSlug})
	if err != nil || len(intoIDs) != 1 {
		t.Fatalf("into namespace ids: %v err=%v", intoIDs, err)
	}
	for _, epID := range []int64{idBefore, idAfter} {
		var onTarget int64
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM episodes WHERE id = $1 AND namespace_id = $2`, epID, intoIDs[0],
		).Scan(&onTarget); err != nil {
			t.Fatal(err)
		}
		if onTarget != 1 {
			t.Fatalf("episode %d not on target namespace %d", epID, intoIDs[0])
		}
	}

	collision, err := br.CreateNamespace(ctx, prefix+"/motion_control", "duplicate", "")
	if err != nil {
		t.Fatal(err)
	}
	if !collision.ExistingSlug || collision.Slug != intoSlug {
		t.Fatalf("collision result = %+v, want existing %s", collision, intoSlug)
	}
}

func createNamespaceRow(ctx context.Context, pool *pgxpool.Pool, slug string) (int64, error) {
	var id int64
	norm := brain.NormalizeNamespaceSlug(slug)
	err := pool.QueryRow(ctx,
		`INSERT INTO namespaces (slug, name, normalized_slug) VALUES ($1, $1, $2)
		 ON CONFLICT (slug) DO UPDATE SET updated_at = now() RETURNING id`,
		slug, norm,
	).Scan(&id)
	return id, err
}