package appauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type metricsHandler struct {
	db *pgxpool.Pool
}

func NewMetricsHandler(db *pgxpool.Pool) http.Handler {
	return &metricsHandler{db: db}
}

func (h *metricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	registrations, err := h.countRegistrations(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	activeUsers, err := h.countActiveUsers(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	totalTasks, err := h.countTotalTasks(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	avgTasks := 0.0
	if registrations > 0 {
		avgTasks = float64(totalTasks) / float64(registrations)
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w,
		"# HELP registrations_total Total number of registered users.\n"+
			"# TYPE registrations_total gauge\n"+
			"registrations_total %d\n"+
			"# HELP active_users_total Total number of active users by unexpired refresh sessions.\n"+
			"# TYPE active_users_total gauge\n"+
			"active_users_total %d\n"+
			"# HELP todo_tasks_total Total number of non-deleted todo tasks.\n"+
			"# TYPE todo_tasks_total gauge\n"+
			"todo_tasks_total %d\n"+
			"# HELP tasks_per_user_avg Average number of todo tasks per registered user.\n"+
			"# TYPE tasks_per_user_avg gauge\n"+
			"tasks_per_user_avg %.6f\n",
		registrations,
		activeUsers,
		totalTasks,
		avgTasks,
	)
}

func (h *metricsHandler) countRegistrations(ctx context.Context) (int64, error) {
	var count int64
	err := h.db.QueryRow(ctx, `SELECT COUNT(1) FROM users`).Scan(&count)
	return count, err
}

func (h *metricsHandler) countActiveUsers(ctx context.Context) (int64, error) {
	var count int64
	err := h.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM refresh_sessions
		WHERE revoked_at IS NULL
		  AND expires_at > now()
		  AND absolute_expires_at > now()
	`).Scan(&count)
	return count, err
}

func (h *metricsHandler) countTotalTasks(ctx context.Context) (int64, error) {
	var count int64
	err := h.db.QueryRow(ctx, `SELECT COUNT(1) FROM todo_tasks WHERE deleted_at_ms IS NULL`).Scan(&count)
	return count, err
}
