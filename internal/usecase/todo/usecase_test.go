package todo

import (
	domain "auth_service/internal/domain/todo"
	"context"
	"testing"
)

type repoMock struct{ applied bool }

func (m *repoMock) ApplyEventTx(context.Context, string, domain.SyncEvent) (bool, error) {
	return m.applied, nil
}
func (m *repoMock) ListTasks(context.Context, string, int, int64, string, bool) ([]domain.Task, int64, string, error) {
	return []domain.Task{{ID: "a", UpdatedAt: 10}}, 0, "", nil
}
func (m *repoMock) CountDoneInRange(context.Context, string, int64, int64) (int, error) {
	return 0, nil
}

func TestValidation(t *testing.T) {
	uc := New(&repoMock{applied: true})
	_, _ = uc.ApplyChanges(context.Background(), "u", []domain.SyncEvent{{ID: "bad"}})
}
func TestIdempotency(t *testing.T) {
	uc := New(&repoMock{applied: false})
	res, _ := uc.ApplyChanges(context.Background(), "u", []domain.SyncEvent{{ID: "123e4567-e89b-12d3-a456-426614174000", Entity: "todo-task", EntityID: "123e4567-e89b-12d3-a456-426614174000", Type: "create", CreatedAt: 1, Payload: domain.Task{ID: "123e4567-e89b-12d3-a456-426614174000", Title: "t", Status: "active", CreatedAt: 1, UpdatedAt: 1}}})
	if res[0].Status != "skipped" {
		t.Fatal()
	}
}
func TestPagination(t *testing.T) {
	uc := New(&repoMock{applied: true})
	_, _, err := uc.List(context.Background(), "u", 10, "1|id", false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteEventWithoutTitleAndStatus(t *testing.T) {
	uc := New(&repoMock{applied: true})
	res, _ := uc.ApplyChanges(context.Background(), "u", []domain.SyncEvent{{
		ID:        "123e4567-e89b-12d3-a456-426614174111",
		Entity:    "todo-task",
		EntityID:  "123e4567-e89b-12d3-a456-426614174222",
		Type:      "delete",
		CreatedAt: 1,
		Payload: domain.Task{
			ID:        "123e4567-e89b-12d3-a456-426614174222",
			CreatedAt: 1,
			UpdatedAt: 1,
		},
	}})
	if res[0].Status != "applied" {
		t.Fatalf("expected applied, got %s (%s)", res[0].Status, res[0].Error)
	}
}

func TestDeleteEventWithoutPayloadTimestamps(t *testing.T) {
	uc := New(&repoMock{applied: true})
	res, _ := uc.ApplyChanges(context.Background(), "u", []domain.SyncEvent{{
		ID:        "123e4567-e89b-12d3-a456-426614174333",
		Entity:    "todo-task",
		EntityID:  "123e4567-e89b-12d3-a456-426614174444",
		Type:      "delete",
		CreatedAt: 1779098777488,
		Payload: domain.Task{
			ID: "123e4567-e89b-12d3-a456-426614174444",
		},
	}})
	if res[0].Status != "applied" {
		t.Fatalf("expected applied, got %s (%s)", res[0].Status, res[0].Error)
	}
}
