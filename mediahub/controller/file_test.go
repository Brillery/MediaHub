package controller

import (
	"bytes"
	"crypto/md5"
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
	called  bool
	dstPath string
}

func (s *fakeStorage) Upload(_ io.Reader, _ []byte, dstPath string) (string, error) {
	s.called = true
	s.dstPath = dstPath
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
