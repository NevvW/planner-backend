package authusecase

import (
	"context"
	"errors"
	"testing"

	domain "auth_service/internal/domain/auth"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestUser_Register(t *testing.T) {
	t.Parallel()

	type fields struct {
		users  func(m *MockUserRepository)
		hasher func(m *MockPasswordHasher)
	}
	type args struct {
		email    string
		password string
		login    string
	}

	ctx := context.Background()
	id := uuid.New()
	hash := []byte("hash")
	hashFail := errors.New("hash fail")
	dbDown := errors.New("db down")

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantID  uuid.UUID
		wantErr error
	}{
		{
			name:    "bad input: empty email",
			fields:  fields{},
			args:    args{email: "", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrBadInput,
		},
		{
			name:    "bad input: empty password",
			fields:  fields{},
			args:    args{email: "a@b.com", password: "", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrBadInput,
		},
		{
			name:    "bad input: email only spaces -> trimmed to empty",
			fields:  fields{},
			args:    args{email: "   ", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrBadInput,
		},
		{
			name: "hasher hash error",
			fields: fields{
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().
						Hash("p").
						Return(nil, hashFail)
				},
			},
			args:    args{email: "a@b.com", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: hashFail,
		},
		{
			name: "repo create user returns ErrEmailAlreadyUsed (wrapped) -> mapped",
			fields: fields{
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().Hash("p").Return(hash, nil)
				},
				users: func(m *MockUserRepository) {
					// email should be normalized (trim)
					m.EXPECT().
						CreateUser(gomock.Any(), "a@b.com", "user_login", hash).
						Return(nil, errors.Join(ErrEmailAlreadyUsed, errors.New("db")))
				},
			},
			args:    args{email: "  a@b.com  ", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrEmailAlreadyUsed,
		},
		{
			name: "repo create user generic error -> passthrough",
			fields: fields{
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().Hash("p").Return(hash, nil)
				},
				users: func(m *MockUserRepository) {
					m.EXPECT().
						CreateUser(gomock.Any(), "a@b.com", "user_login", hash).
						Return(nil, dbDown)
				},
			},
			args:    args{email: "a@b.com", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: dbDown,
		},
		{
			name: "success",
			fields: fields{
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().Hash("p").Return(hash, nil)
				},
				users: func(m *MockUserRepository) {
					m.EXPECT().
						CreateUser(gomock.Any(), "a@b.com", "user_login", hash).
						Return(&domain.User{ID: id}, nil)
				},
			},
			args:    args{email: " a@b.com ", password: "p", login: "user_login"},
			wantID:  id,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			users := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)

			if tt.fields.users != nil {
				tt.fields.users(users)
			}
			if tt.fields.hasher != nil {
				tt.fields.hasher(hasher)
			}

			uc := NewUser(users, hasher)

			gotID, err := uc.Register(ctx, tt.args.email, tt.args.password, tt.args.login)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			} else {
				// сравниваем по errors.Is, чтобы поддержать враппинг
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			}

			if gotID != tt.wantID {
				t.Fatalf("expected id %v, got %v", tt.wantID, gotID)
			}
		})
	}
}

func TestUser_Authenticate(t *testing.T) {
	t.Parallel()

	type fields struct {
		users  func(m *MockUserRepository)
		hasher func(m *MockPasswordHasher)
	}
	type args struct {
		email    string
		password string
		login    string
	}

	ctx := context.Background()
	id := uuid.New()
	passHash := []byte("hash")
	dbDown := errors.New("db down")
	compareErr := errors.New("compare fail")
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantID  uuid.UUID
		wantErr error
	}{
		{
			name:    "bad input: empty email",
			fields:  fields{},
			args:    args{email: "", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrBadInput,
		},
		{
			name:    "bad input: empty password",
			fields:  fields{},
			args:    args{email: "a@b.com", password: "", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrBadInput,
		},
		{
			name: "repo GetUserByEmail ErrUserNotFound -> mapped to ErrInvalidCredentials",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(nil, ErrUserNotFound)
				},
			},
			args:    args{email: " a@b.com ", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "repo GetUserByEmail generic error -> passthrough",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(nil, dbDown)
				},
			},
			args:    args{email: "a@b.com", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: dbDown,
		},
		{
			name: "hasher compare error -> passthrough",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(&domain.User{ID: id, PasswordHash: passHash}, nil)
				},
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().
						Compare(passHash, "p").
						Return(false, compareErr)
				},
			},
			args:    args{email: "a@b.com", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: compareErr,
		},
		{
			name: "wrong password -> ErrInvalidCredentials",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(&domain.User{ID: id, PasswordHash: passHash}, nil)
				},
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().
						Compare(passHash, "p").
						Return(false, nil)
				},
			},
			args:    args{email: "a@b.com", password: "p", login: "user_login"},
			wantID:  uuid.Nil,
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "success",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByEmail(gomock.Any(), "a@b.com").
						Return(&domain.User{ID: id, PasswordHash: passHash}, nil)
				},
				hasher: func(m *MockPasswordHasher) {
					m.EXPECT().
						Compare(passHash, "p").
						Return(true, nil)
				},
			},
			args:    args{email: " a@b.com ", password: "p", login: "user_login"},
			wantID:  id,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			users := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl)

			if tt.fields.users != nil {
				tt.fields.users(users)
			}
			if tt.fields.hasher != nil {
				tt.fields.hasher(hasher)
			}

			uc := NewUser(users, hasher)

			gotID, err := uc.Authenticate(ctx, tt.args.email, tt.args.password)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			} else {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			}

			if gotID != tt.wantID {
				t.Fatalf("expected id %v, got %v", tt.wantID, gotID)
			}
		})
	}
}

func TestUser_GetByID(t *testing.T) {
	t.Parallel()

	type fields struct {
		users func(m *MockUserRepository)
	}
	type args struct {
		id uuid.UUID
	}

	ctx := context.Background()
	id := uuid.New()
	dbFailErr := errors.New("db fail")
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantID  uuid.UUID
		wantErr error
	}{
		{
			name:    "bad input: nil uuid",
			fields:  fields{},
			args:    args{id: uuid.Nil},
			wantID:  uuid.Nil,
			wantErr: ErrBadInput,
		},
		{
			name: "repo ErrUserNotFound -> ErrUserNotFound",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByID(gomock.Any(), id).
						Return(nil, ErrUserNotFound)
				},
			},
			args:    args{id: id},
			wantID:  uuid.Nil,
			wantErr: ErrUserNotFound,
		},
		{
			name: "repo generic error -> passthrough",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByID(gomock.Any(), id).
						Return(nil, dbFailErr)
				},
			},
			args:    args{id: id},
			wantID:  uuid.Nil,
			wantErr: dbFailErr,
		},
		{
			name: "success",
			fields: fields{
				users: func(m *MockUserRepository) {
					m.EXPECT().
						GetUserByID(gomock.Any(), id).
						Return(&domain.User{ID: id}, nil)
				},
			},
			args:    args{id: id},
			wantID:  id,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			users := NewMockUserRepository(ctrl)
			hasher := NewMockPasswordHasher(ctrl) // не нужен, но конструктор требует

			if tt.fields.users != nil {
				tt.fields.users(users)
			}

			uc := NewUser(users, hasher)

			gotID, err := uc.GetByID(ctx, tt.args.id)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			} else {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			}

			if gotID != tt.wantID {
				t.Fatalf("expected id %v, got %v", tt.wantID, gotID)
			}
		})
	}
}
