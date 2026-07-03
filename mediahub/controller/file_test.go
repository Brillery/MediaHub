package controller

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"enterprise-project1-mediahub/mediahub/middleware"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/storage"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	called         bool
	dstPath        string
	readerSeekable bool
	uploadedBytes  []byte
}

func (s *fakeStorage) Upload(r io.Reader, _ []byte, dstPath string) (string, error) {
	s.called = true
	s.dstPath = dstPath
	_, s.readerSeekable = r.(io.Seeker)
	content, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s.uploadedBytes = content
	return "https://img.example.com/uploaded.jpg", nil
}

type fakeShortener struct {
	called      bool
	originalURL string
	userID      int64
	isPublic    bool
}

func (s *fakeShortener) Generate(_ context.Context, originalURL string, userID int64, isPublic bool) (string, error) {
	s.called = true
	s.originalURL = originalURL
	s.userID = userID
	s.isPublic = isPublic
	return "https://short.example/abc", nil
}

func TestUploadStoresImageAndReturnsShortURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeStorage := &fakeStorage{}
	shortener := &fakeShortener{}
	controller := NewControllerWithShortener(&fakeStorageFactory{storage: fakeStorage}, log.NewLogger(), &config.Config{}, shortener)
	body, contentType := multipartBodyWithFields(t, map[string]string{"user_name": "mediahub-user"}, "file", "avatar.txt", jpegImageBytes(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file/upload", body)
	req.Header.Set("Content-Type", contentType)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set(middleware.AuthUserIDKey, int64(42))

	controller.Upload(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var payload struct {
		URL      string `json:"url"`
		UserName string `json:"user_name"`
		Msg      string `json:"msg"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json response decode failed: %v", err)
	}
	if payload.URL != "https://short.example/abc" {
		t.Fatalf("response url = %q, want shortener result", payload.URL)
	}
	if payload.UserName != "mediahub-user" {
		t.Fatalf("response user_name = %q, want multipart field value", payload.UserName)
	}
	if !fakeStorage.called {
		t.Fatal("storage should be called for a valid image")
	}
	if !strings.HasPrefix(fakeStorage.dstPath, "/42/") || !strings.HasSuffix(fakeStorage.dstPath, ".jpg") {
		t.Fatalf("storage path = %q, want user scoped .jpg path", fakeStorage.dstPath)
	}
	if !shortener.called {
		t.Fatal("shortener should be called after storage upload")
	}
	if shortener.originalURL != "https://img.example.com/uploaded.jpg" {
		t.Fatalf("shortener original url = %q, want storage url", shortener.originalURL)
	}
	if shortener.userID != 42 || shortener.isPublic {
		t.Fatalf("shortener scope userID/isPublic = %d/%v, want 42/false", shortener.userID, shortener.isPublic)
	}
}

func TestUploadUsesSeekableTempFileAndCleansIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeStorage := &fakeStorage{}
	shortener := &fakeShortener{}
	controller := NewControllerWithShortener(&fakeStorageFactory{storage: fakeStorage}, log.NewLogger(), &config.Config{}, shortener)
	controller.uploadTempDir = t.TempDir()
	imageContent := jpegImageBytes(t)
	body, contentType := multipartBody(t, "file", "avatar.jpg", imageContent)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file/upload", body)
	req.Header.Set("Content-Type", contentType)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	controller.Upload(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	if !fakeStorage.readerSeekable {
		t.Fatal("storage should receive a seekable temp file reader")
	}
	if !bytes.Equal(fakeStorage.uploadedBytes, imageContent) {
		t.Fatalf("uploaded bytes length = %d, want original length %d", len(fakeStorage.uploadedBytes), len(imageContent))
	}
	entries, err := os.ReadDir(controller.uploadTempDir)
	if err != nil {
		t.Fatalf("ReadDir temp dir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp dir entries = %d, want 0 after cleanup", len(entries))
	}
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

func TestBuildUploadFilePathUsesDetectedImageExtension(t *testing.T) {
	content := jpegImageBytes(t)
	meta, ok := detectUploadImage(bytes.NewReader(content))
	if !ok {
		t.Fatal("detectUploadImage returned false for a valid jpeg")
	}

	digest := calMD5Digest(content)
	got := buildUploadFilePath(42, digest, meta)
	want := fmt.Sprintf("/42/%x.jpg", digest)
	if got != want {
		t.Fatalf("file path = %q, want %q", got, want)
	}
}

func multipartBody(t *testing.T, fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	return multipartBodyWithFields(t, nil, fieldName, fileName, content)
}

func multipartBodyWithFields(t *testing.T, fields map[string]string, fieldName, fileName string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField %s failed: %v", key, err)
		}
	}
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

func jpegImageBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode failed: %v", err)
	}
	return buf.Bytes()
}
