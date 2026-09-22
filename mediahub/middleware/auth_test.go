package middleware

import (
	"context"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// authTransport 在进程内模拟用户中心，禁止测试访问真实账户服务。
type authTransport func(*http.Request) (*http.Response, error)

func (f authTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthRejectsInvalidUpstreamResponses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("dependOn:\n  user:\n    address: https://auth.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config.InitConfig(path)
	old := httpClient
	t.Cleanup(func() { httpClient = old })
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   int
	}{
		{"forbidden", 403, `{"id":42}`, 401},
		{"upstream unavailable", 503, `{"id":42}`, 500},
		{"missing identity", 200, `{}`, 500},
		{"zero identity", 200, `{"id":0}`, 500},
		{"negative identity", 200, `{"id":-1}`, 500},
		{"valid", 200, `{"id":42}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpClient = &http.Client{Transport: authTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			router := gin.New()
			router.Use(Auth())
			router.GET("/", func(c *gin.Context) { c.Status(200) })
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestAuthPreservesTokenQueryValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("dependOn:\n  user:\n    address: https://auth.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config.InitConfig(path)
	old := httpClient
	t.Cleanup(func() { httpClient = old })
	token := "abc+def&role=admin#fragment"
	httpClient = &http.Client{Transport: authTransport(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("access_token"); got != token {
			t.Errorf("token = %q, want %q", got, token)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":42}`))}, nil
	})}
	router := gin.New()
	router.Use(Auth())
	router.GET("/", func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(httptest.NewRecorder(), req)
}

func TestAuthResponseBudgetAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("dependOn:\n  user:\n    address: https://auth.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config.InitConfig(path)
	old := httpClient
	t.Cleanup(func() { httpClient = old })
	httpClient = &http.Client{Transport: authTransport(func(r *http.Request) (*http.Response, error) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("auth request has no deadline")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(strings.Repeat(" ", maxAuthResponseBytes) + `{"id":42}`))}, nil
	})}
	if _, err := checkAuth(context.Background(), "secret"); err == nil {
		t.Fatal("oversized response accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	httpClient = &http.Client{Transport: authTransport(func(r *http.Request) (*http.Response, error) {
		if r.Context().Err() != context.Canceled {
			t.Error("request cancellation not propagated")
		}
		return nil, fmt.Errorf("transport failed with secret")
	})}
	if _, err := checkAuth(ctx, "secret"); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
