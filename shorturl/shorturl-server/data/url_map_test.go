package data

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"shorturl/pkg/log"
)

func TestGetByIDReturnsNilWhenRowMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("select original_url from url_map where id = \\?").
		WithArgs(int64(404)).
		WillReturnError(sql.ErrNoRows)

	data := newUrlMapData(log.NewLogger(), db, "url_map")
	entity, err := data.GetByID(404)
	if err != nil {
		t.Fatalf("GetByID error = %v, want nil", err)
	}
	if entity != nil {
		t.Fatalf("entity = %#v, want nil", entity)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetByOriginalReturnsOriginalURLForCacheBackfill(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer db.Close()

	originalURL := "https://img.example.com/a.jpg"
	mock.ExpectQuery("select id, short_key, original_url from url_map where original_url = \\?").
		WithArgs(originalURL).
		WillReturnRows(sqlmock.NewRows([]string{"id", "short_key", "original_url"}).
			AddRow(int64(88), "1q", originalURL))

	data := newUrlMapData(log.NewLogger(), db, "url_map")
	entity, err := data.GetByOriginal(originalURL)
	if err != nil {
		t.Fatalf("GetByOriginal error = %v, want nil", err)
	}
	if entity.ID != 88 || entity.ShortKey != "1q" || entity.OriginalUrl != originalURL {
		t.Fatalf("entity = %#v, want id/short_key/original_url populated", entity)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
