package sqlstore

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDomainStoreListByProjectIncludesLinkReadyAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDomainStore(db)
	ctx := context.Background()

	created := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	updated := created.Add(2 * time.Hour)
	published := created.Add(-time.Hour)
	linkUpdated := created.Add(30 * time.Minute)
	linkReady := created.Add(45 * time.Minute)

	rows := sqlmock.NewRows([]string{
		"id",
		"project_id",
		"server_id",
		"url",
		"main_keyword",
		"target_country",
		"target_language",
		"exclude_domains",
		"specific_blacklist",
		"status",
		"last_generation_id",
		"last_success_generation_id",
		"published_at",
		"published_path",
		"file_count",
		"total_size_bytes",
		"deployment_mode",
		"link_anchor_text",
		"link_acceptor_url",
		"link_status",
		"link_updated_at",
		"link_last_task_id",
		"link_file_path",
		"link_anchor_snapshot",
		"link_ready_at",
		"created_at",
		"updated_at",
	}).AddRow(
		"dom-1",
		"proj-1",
		sql.NullString{String: "srv-1", Valid: true},
		"example.com",
		"keyword",
		"se",
		"sv",
		sql.NullString{String: "bad.com", Valid: true},
		sql.NullString{String: "blocked", Valid: true},
		"published",
		sql.NullString{String: "gen-1", Valid: true},
		sql.NullString{String: "gen-1", Valid: true},
		published,
		sql.NullString{String: "/server/example.com/", Valid: true},
		10,
		2048,
		sql.NullString{String: "local_mock", Valid: true},
		sql.NullString{String: "anchor", Valid: true},
		sql.NullString{String: "https://target.example", Valid: true},
		sql.NullString{String: "inserted", Valid: true},
		linkUpdated,
		sql.NullString{String: "task-1", Valid: true},
		sql.NullString{String: "index.html", Valid: true},
		sql.NullString{String: "<a>anchor</a>", Valid: true},
		linkReady,
		created,
		updated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, created_at, updated_at FROM domains WHERE project_id=$1 ORDER BY updated_at DESC")).
		WithArgs("proj-1").
		WillReturnRows(rows)

	res, err := store.ListByProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(res))
	}
	if !res[0].LinkReadyAt.Valid {
		t.Fatalf("expected LinkReadyAt to be valid")
	}
	if !res[0].LinkReadyAt.Time.Equal(linkReady) {
		t.Fatalf("unexpected LinkReadyAt: %v", res[0].LinkReadyAt.Time)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDomainStoreListByProjectError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDomainStore(db)
	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, created_at, updated_at FROM domains WHERE project_id=$1 ORDER BY updated_at DESC")).
		WithArgs("proj-1").
		WillReturnError(sql.ErrConnDone)

	if _, err := store.ListByProject(ctx, "proj-1"); err == nil {
		t.Fatalf("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDomainStoreGetIncludesLinkReadyAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDomainStore(db)
	ctx := context.Background()

	created := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	updated := created.Add(2 * time.Hour)
	linkReady := created.Add(45 * time.Minute)

	rows := sqlmock.NewRows([]string{
		"id",
		"project_id",
		"server_id",
		"url",
		"main_keyword",
		"target_country",
		"target_language",
		"exclude_domains",
		"specific_blacklist",
		"status",
		"last_generation_id",
		"last_success_generation_id",
		"published_at",
		"published_path",
		"file_count",
		"total_size_bytes",
		"deployment_mode",
		"link_anchor_text",
		"link_acceptor_url",
		"link_status",
		"link_updated_at",
		"link_last_task_id",
		"link_file_path",
		"link_anchor_snapshot",
		"link_ready_at",
		"created_at",
		"updated_at",
	}).AddRow(
		"dom-1",
		"proj-1",
		sql.NullString{String: "srv-1", Valid: true},
		"example.com",
		"keyword",
		"se",
		"sv",
		sql.NullString{String: "bad.com", Valid: true},
		sql.NullString{String: "blocked", Valid: true},
		"published",
		sql.NullString{String: "gen-1", Valid: true},
		sql.NullString{String: "gen-1", Valid: true},
		created,
		sql.NullString{String: "/server/example.com/", Valid: true},
		10,
		2048,
		sql.NullString{String: "local_mock", Valid: true},
		sql.NullString{String: "anchor", Valid: true},
		sql.NullString{String: "https://target.example", Valid: true},
		sql.NullString{String: "inserted", Valid: true},
		created,
		sql.NullString{String: "task-1", Valid: true},
		sql.NullString{String: "index.html", Valid: true},
		sql.NullString{String: "<a>anchor</a>", Valid: true},
		linkReady,
		created,
		updated,
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, created_at, updated_at FROM domains WHERE id=$1")).
		WithArgs("dom-1").
		WillReturnRows(rows)

	dom, err := store.Get(ctx, "dom-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !dom.LinkReadyAt.Valid {
		t.Fatalf("expected LinkReadyAt to be valid")
	}
	if !dom.LinkReadyAt.Time.Equal(linkReady) {
		t.Fatalf("unexpected LinkReadyAt: %v", dom.LinkReadyAt.Time)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDomainStoreGetError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDomainStore(db)
	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, created_at, updated_at FROM domains WHERE id=$1")).
		WithArgs("dom-1").
		WillReturnError(sql.ErrConnDone)

	if _, err := store.Get(ctx, "dom-1"); err == nil {
		t.Fatalf("expected error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDomainStoreGetByURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDomainStore(db)
	ctx := context.Background()
	now := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id",
		"project_id",
		"server_id",
		"url",
		"main_keyword",
		"target_country",
		"target_language",
		"exclude_domains",
		"specific_blacklist",
		"status",
		"last_generation_id",
		"last_success_generation_id",
		"published_at",
		"published_path",
		"file_count",
		"total_size_bytes",
		"deployment_mode",
		"link_anchor_text",
		"link_acceptor_url",
		"link_status",
		"link_updated_at",
		"link_last_task_id",
		"link_file_path",
		"link_anchor_snapshot",
		"link_ready_at",
		"created_at",
		"updated_at",
	}).AddRow(
		"dom-1",
		"proj-1",
		sql.NullString{String: "srv-1", Valid: true},
		"example.com",
		"kw",
		"se",
		"sv",
		sql.NullString{},
		sql.NullString{},
		"published",
		sql.NullString{},
		sql.NullString{},
		sql.NullTime{},
		sql.NullString{String: "/server/example.com/", Valid: true},
		10,
		2048,
		sql.NullString{String: "local_mock", Valid: true},
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		sql.NullTime{},
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		sql.NullTime{},
		now,
		now,
	)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, project_id, server_id, url, main_keyword, target_country, target_language, exclude_domains, specific_blacklist, status, last_generation_id, last_success_generation_id, published_at, published_path, file_count, total_size_bytes, deployment_mode, link_anchor_text, link_acceptor_url, link_status, link_updated_at, link_last_task_id, link_file_path, link_anchor_snapshot, link_ready_at, created_at, updated_at FROM domains WHERE url=$1")).
		WithArgs("example.com").
		WillReturnRows(rows)

	d, err := store.GetByURL(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetByURL failed: %v", err)
	}
	if d.ID != "dom-1" {
		t.Fatalf("unexpected domain id: %s", d.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDomainStoreUpdateProject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	store := NewDomainStore(db)
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE domains SET project_id=$1, updated_at=NOW() WHERE id=$2")).
		WithArgs("proj-2", "dom-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.UpdateProject(ctx, "dom-1", "proj-2"); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
