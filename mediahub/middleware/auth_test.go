package middleware

import "testing"

func TestAuthContextKeysMatchControllerContract(t *testing.T) {
	t.Parallel()

	if AuthUserIDKey != "user_id" {
		t.Fatalf("AuthUserIDKey = %q, want user_id", AuthUserIDKey)
	}
	if AuthUserNameKey != "user_name" {
		t.Fatalf("AuthUserNameKey = %q, want user_name", AuthUserNameKey)
	}
	if AuthUserAvatarURLKey != "avatar_url" {
		t.Fatalf("AuthUserAvatarURLKey = %q, want avatar_url", AuthUserAvatarURLKey)
	}
}
