package apptodo

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	domain "auth_service/internal/domain/todo"
	todouc "auth_service/internal/usecase/todo"

	"github.com/google/uuid"
)

type AuthGrpcClient interface {
	ValidateAccessToken(ctx context.Context, token string) (string, error)
}

type handler struct {
	uc   *todouc.Usecase
	auth AuthGrpcClient
}

func NewHandler(uc *todouc.Usecase, auth AuthGrpcClient) http.Handler {
	return &handler{uc: uc, auth: auth}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/todo/") {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userID := strings.TrimPrefix(r.URL.Path, "/v1/todo/")
	parts := strings.SplitN(strings.TrimSpace(r.Header.Get("Authorization")), " ", 2)
	if len(parts) != 2 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tokenUserID, err := h.auth.ValidateAccessToken(r.Context(), parts[1])
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if tokenUserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		changes := make([]domain.SyncEvent, 0)
		var req struct {
			Changes []domain.SyncEvent `json:"changes"`
		}
		if err := json.Unmarshal(raw, &req); err == nil {
			changes = req.Changes
		}

		if len(changes) == 0 {
			var task domain.Task
			if err := json.Unmarshal(raw, &task); err == nil && task.ID != "" {
				ts := task.UpdatedAt
				if ts <= 0 {
					ts = task.CreatedAt
				}
				if ts <= 0 && task.DeletedAt != nil {
					ts = *task.DeletedAt
				}
				eventType := "create"
				if task.DeletedAt != nil {
					eventType = "delete"
				}
				changes = []domain.SyncEvent{{
					ID:        uuid.NewString(),
					CreatedAt: ts,
					Entity:    "todo-task",
					EntityID:  task.ID,
					Type:      eventType,
					Payload:   task,
				}}
			}
		}

		if len(changes) == 0 {
			http.Error(w, "changes required", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		results, err := h.uc.ApplyChanges(r.Context(), userID, changes)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
		return
	}
	if r.Method == http.MethodGet {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		cursor := r.URL.Query().Get("cursor")
		includeDeleted := r.URL.Query().Get("includeDeleted") == "true"
		items, next, err := h.uc.List(r.Context(), userID, limit, cursor, includeDeleted)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "nextCursor": next})
		return
	}
	http.NotFound(w, r)
}
