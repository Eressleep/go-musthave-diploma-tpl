package auth

import (
	"strings"
	"testing"
)

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password in test helper: %v", err)
	}
	return hash
}

func splitToken(token string) []string {
	return strings.Split(token, ".")
}

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "обычный пароль",
			password: "correct horse battery staple",
			wantErr:  false,
		},
		{
			name:     "короткий пароль",
			password: "12345678",
			wantErr:  false,
		},
		{
			name:     "длинный пароль со спецсимволами",
			password: "P@ssw0rd!with#Special$Chars%Very^Long&String*For(Testing)123",
			wantErr:  false,
		},
		{
			name:     "пустой пароль",
			password: "",
			wantErr:  true,
		},
		{
			name:     "unicode пароль",
			password: "парольНаРусском2024!",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hash == "" {
				t.Error("hash is empty")
			}
			if hash == tt.password {
				t.Error("hash equals original password")
			}
			// Хэш bcrypt всегда начинается с "$2a$"
			if len(hash) < 4 || hash[:4] != "$2a$" {
				t.Errorf("invalid bcrypt hash format: %s", hash)
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "правильный пароль",
			password: "my-secret-password",
			hash:     mustHashPassword(t, "my-secret-password"),
			want:     true,
		},
		{
			name:     "неправильный пароль",
			password: "wrong-password",
			hash:     mustHashPassword(t, "my-secret-password"),
			want:     false,
		},
		{
			name:     "пустой пароль",
			password: "",
			hash:     mustHashPassword(t, "some-password"),
			want:     false,
		},
		{
			name:     "пустой хэш",
			password: "password",
			hash:     "",
			want:     false,
		},
		{
			name:     "оба пустые",
			password: "",
			hash:     "",
			want:     false,
		},
		{
			name:     "невалидный хэш",
			password: "password",
			hash:     "not-a-valid-hash",
			want:     false,
		},
		{
			name:     "пароль с пробелами",
			password: " my password ",
			hash:     mustHashPassword(t, " my password "),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPassword(tt.hash, tt.password)
			if got != tt.want {
				t.Errorf("CheckPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueToken(t *testing.T) {
	secret := "test-secret-key-for-jwt-signing-12345"
	mgr := NewManager(secret)

	tests := []struct {
		name   string
		userID int64
	}{
		{
			name:   "обычный пользователь",
			userID: 42,
		},
		{
			name:   "ID 1",
			userID: 1,
		},
		{
			name:   "большой ID",
			userID: 999999999,
		},
		{
			name:   "отрицательный ID",
			userID: -1,
		},
		{
			name:   "нулевой ID",
			userID: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := mgr.Issue(tt.userID)
			if err != nil {
				t.Fatalf("Issue() error: %v", err)
			}
			if token == "" {
				t.Error("token is empty")
			}

			// Проверяем, что токен состоит из трёх частей, разделённых точками
			parts := splitToken(token)
			if len(parts) != 3 {
				t.Errorf("token has %d parts, want 3", len(parts))
			}
		})
	}
}
