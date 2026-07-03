package controller

import (
	"bytes"
	"crypto/md5"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/storage"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeStorageFactory struct {
	storage *fakeStorage
}

func (f *fakeStorageFactory) CreateStorage() storage.Storage {
	return f.storage
}

type fakeStorage struct {
	called bool
}

func (s *fakeStorage) Upload(_ io.Reader, _ []byte, _ string) (string, error) {
	s.called = true
	return "https://img.example.com/uploaded.jpg", nil
}

func TestUploadRejectsInvalidImageBeforeStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeStorage{}
	controller := NewController(&fakeStorageFactory{storage: fake}, log.NewLogger(), &config.Config{})
	body, contentType := multipartBody(t, "file", "not-image.txt", []byte("not an image"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file/upload", body)
	req.Header.Set("Content-Type", contentType)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	controller.Upload(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if fake.called {
		t.Fatal("storage should not be called when image validation fails")
	}
}

func TestUploadRejectsOversizedFileBeforeStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakeStorage{}
	controller := NewController(&fakeStorageFactory{storage: fake}, log.NewLogger(), &config.Config{})
	content := bytes.Repeat([]byte("x"), MaxUploadBytes+1)
	body, contentType := multipartBody(t, "file", "large.jpg", content)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file/upload", body)
	req.Header.Set("Content-Type", contentType)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	controller.Upload(ctx)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if fake.called {
		t.Fatal("storage should not be called when upload exceeds size limit")
	}
}

func TestCalMD5Digest(t *testing.T) {
	got := calMD5Digest([]byte("mediahub"))
	want := md5.Sum([]byte("mediahub"))
	if !bytes.Equal(got, want[:]) {
		t.Fatalf("digest = %x, want %x", got, want)
	}
}

func multipartBody(t *testing.T, fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart content failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}
	return body, writer.FormDataContentType()
}
