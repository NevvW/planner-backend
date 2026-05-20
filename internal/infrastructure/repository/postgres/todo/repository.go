package todo

import (
	"context"
	"fmt"

	domain "auth_service/internal/domain/todo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ApplyEventTx(ctx context.Context, userID string, e domain.SyncEvent) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		_ = tx.Rollback(ctx)
	}(tx, ctx)
	ct, err := tx.Exec(ctx, "INSERT INTO todo_sync_events(id,user_id,entity_id,created_at_ms) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", e.ID, userID, e.EntityID, e.CreatedAt)
	if err != nil {
		return false, err
	}
	if ct.RowsAffected() == 0 {
		_ = tx.Commit(ctx)
		return false, nil
	}
	if e.Type == "delete" {
		deletedAt := e.CreatedAt
		_, err = tx.Exec(ctx, `INSERT INTO todo_tasks (id,user_id,title,description,start_ms,deadline_ms,status,priority,tags,type,task_done_time_ms,created_at_ms,updated_at_ms,deleted_at_ms,version)
VALUES($1,$2,'', '',NULL,NULL,'active',NULL,'{}',NULL,NULL,$3,$3,$3,0)
ON CONFLICT (id,user_id) DO UPDATE SET deleted_at_ms=EXCLUDED.deleted_at_ms,updated_at_ms=EXCLUDED.updated_at_ms`,
			e.Payload.ID, userID, deletedAt)
		if err != nil {
			return false, err
		}
		return true, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `INSERT INTO todo_tasks (id,user_id,title,description,start_ms,deadline_ms,status,priority,tags,type,task_done_time_ms,created_at_ms,updated_at_ms,deleted_at_ms,version)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (id,user_id) DO UPDATE SET title=EXCLUDED.title,description=EXCLUDED.description,start_ms=EXCLUDED.start_ms,deadline_ms=EXCLUDED.deadline_ms,status=EXCLUDED.status,priority=EXCLUDED.priority,tags=EXCLUDED.tags,type=EXCLUDED.type,task_done_time_ms=EXCLUDED.task_done_time_ms,created_at_ms=EXCLUDED.created_at_ms,updated_at_ms=EXCLUDED.updated_at_ms,deleted_at_ms=EXCLUDED.deleted_at_ms,version=EXCLUDED.version`, e.Payload.ID, userID, e.Payload.Title, e.Payload.Description, e.Payload.Start, e.Payload.Deadline, e.Payload.Status, e.Payload.Priority, e.Payload.Tags, e.Payload.Type, e.Payload.TaskDoneTime, e.Payload.CreatedAt, e.Payload.UpdatedAt, e.Payload.DeletedAt, e.Payload.Version)
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (r *Repository) ListTasks(ctx context.Context, userID string, limit int, updatedAt int64, id string, includeDeleted bool) ([]domain.Task, int64, string, error) {
	q := `
SELECT
	id::text,
	title,
	description,
	start_ms,
	deadline_ms,
	status,
	priority,
	tags,
	type,
	task_done_time_ms,
	created_at_ms,
	updated_at_ms,
	deleted_at_ms,
	version
FROM todo_tasks
WHERE user_id = $1`

	args := []any{userID}

	if !includeDeleted {
		q += " AND deleted_at_ms IS NULL"
	}

	if updatedAt > 0 {
		args = append(args, updatedAt, id)
		q += " AND (updated_at_ms < $2 OR (updated_at_ms = $2 AND id::text > $3))"
	}

	args = append(args, limit+1)
	q += fmt.Sprintf(" ORDER BY updated_at_ms DESC, id ASC LIMIT $%d", len(args))

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, "", err
	}
	defer rows.Close()

	items := []domain.Task{}
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(
			&t.ID,
			&t.Title,
			&t.Description,
			&t.Start,
			&t.Deadline,
			&t.Status,
			&t.Priority,
			&t.Tags,
			&t.Type,
			&t.TaskDoneTime,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.DeletedAt,
			&t.Version,
		); err != nil {
			return nil, 0, "", err
		}
		items = append(items, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, "", err
	}

	if len(items) <= limit {
		return items, 0, "", nil
	}

	n := items[limit-1]
	return items[:limit], n.UpdatedAt, n.ID, nil
}

func (r *Repository) CountDoneInRange(ctx context.Context, userID string, fromMs, toMs int64) (int, error) {
	var c int
	err := r.db.QueryRow(ctx, `SELECT count(1) FROM todo_tasks WHERE user_id=$1 AND status='done' AND task_done_time_ms IS NOT NULL AND task_done_time_ms >= $2 AND task_done_time_ms < $3 AND deleted_at_ms IS NULL`, userID, fromMs, toMs).Scan(&c)
	return c, err
}
