package todo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	domain "auth_service/internal/domain/todo"
)

type Repository interface {
	ApplyEventTx(context.Context, string, domain.SyncEvent) (bool, error)
	ListTasks(context.Context, string, int, int64, string, bool) ([]domain.Task, int64, string, error)
	CountDoneInRange(context.Context, string, int64, int64) (int, error)
}

type Usecase struct{ repo Repository }

func New(r Repository) *Usecase { return &Usecase{repo: r} }

var uuidRe = regexp.MustCompile(`^[a-fA-F0-9-]{36}$`)

type EventResult struct {
	EventID  string `json:"eventId,omitempty"`
	EntityID string `json:"entityId,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
}

func validate(e domain.SyncEvent) error {
	if !uuidRe.MatchString(e.ID) || !uuidRe.MatchString(e.EntityID) || !uuidRe.MatchString(e.Payload.ID) {
		return errors.New("bad uuid")
	}
	if e.Entity != "todo-task" || e.EntityID != e.Payload.ID {
		return errors.New("bad entity")
	}
	if e.Type != "create" && e.Type != "update" && e.Type != "delete" {
		return errors.New("bad type")
	}
	if e.Type != "delete" {
		if e.Payload.Title == "" {
			return errors.New("title required")
		}
		if e.Payload.Status != "active" && e.Payload.Status != "done" {
			return errors.New("bad status")
		}
	}
	if e.Payload.Type != nil && *e.Payload.Type != "Задача" {
		return errors.New("bad task type")
	}
	if e.Payload.Priority != nil && *e.Payload.Priority != "Важно" && *e.Payload.Priority != "Не важно" {
		return errors.New("bad priority")
	}
	if e.CreatedAt <= 0 {
		return errors.New("bad timestamps")
	}
	if e.Type != "delete" && (e.Payload.UpdatedAt <= 0 || e.Payload.CreatedAt <= 0) {
		return errors.New("bad timestamps")
	}
	return nil
}

func (u *Usecase) ApplyChanges(ctx context.Context, userID string, changes []domain.SyncEvent) ([]EventResult, error) {
	res := make([]EventResult, 0, len(changes))
	for _, e := range changes {
		if err := validate(e); err != nil {
			res = append(res, EventResult{EventID: e.ID, EntityID: e.EntityID, Status: "failed", Error: err.Error()})
			continue
		}
		applied, err := u.repo.ApplyEventTx(ctx, userID, e)
		if err != nil {
			res = append(res, EventResult{EventID: e.ID, EntityID: e.EntityID, Status: "failed", Error: err.Error()})
			continue
		}
		st := "applied"
		if !applied {
			st = "skipped"
		}
		res = append(res, EventResult{EventID: e.ID, EntityID: e.EntityID, Status: st})
	}
	return res, nil
}

func encodeCursor(updatedAt int64, id string) string { return fmt.Sprintf("%d|%s", updatedAt, id) }
func decodeCursor(c string) (int64, string, error) {
	if c == "" {
		return 0, "", nil
	}
	p := strings.Split(c, "|")
	if len(p) != 2 {
		return 0, "", errors.New("bad cursor")
	}
	ts, err := strconv.ParseInt(p[0], 10, 64)
	return ts, p[1], err
}

func (u *Usecase) List(ctx context.Context, userID string, limit int, cursor string, includeDeleted bool) ([]domain.Task, *string, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	ts, id, err := decodeCursor(cursor)
	if err != nil {
		return nil, nil, err
	}
	items, nTS, nID, err := u.repo.ListTasks(ctx, userID, limit, ts, id, includeDeleted)
	if err != nil {
		return nil, nil, err
	}
	if nID == "" {
		return items, nil, nil
	}
	n := encodeCursor(nTS, nID)
	return items, &n, nil
}
