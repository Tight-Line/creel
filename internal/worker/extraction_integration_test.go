package worker

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tight-Line/creel/internal/config"
	"github.com/Tight-Line/creel/internal/store"
)

func setupIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pgCfg := config.PostgresConfigFromEnv()
	if pgCfg == nil {
		t.Skip("CREEL_POSTGRES_HOST not set; skipping integration test")
	}

	ctx := context.Background()

	if err := store.EnsureSchema(ctx, pgCfg.BaseURL(), pgCfg.Schema); err != nil {
		t.Fatalf("ensuring schema: %v", err)
	}

	migrationsDir := "../../migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Skip("migrations directory not found")
	}
	if err := store.RunMigrations(pgCfg.URL(), migrationsDir); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgCfg.URL())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return pool
}

func TestExtractionWorker_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	jobStore := store.NewJobStore(pool)

	// Create a topic.
	topic, err := topicStore.Create(ctx, fmt.Sprintf("extraction-test-%d", time.Now().UnixNano()),
		"Extraction Test", "", "system:test", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	// Create a pending document.
	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "test-extraction", "Test Extraction", "reference", "pending", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	// Save content.
	htmlContent := `<html><body><h1>Hello World</h1><p>This is a test.</p></body></html>`
	if err := docStore.SaveContent(ctx, doc.ID, []byte(htmlContent), "text/html"); err != nil {
		t.Fatalf("saving content: %v", err)
	}

	// Create extraction job.
	job, err := jobStore.Create(ctx, doc.ID, "extraction")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	// Run extraction worker.
	w := NewExtractionWorker(docStore, jobStore)
	if w.Type() != "extraction" {
		t.Errorf("Type() = %q, want extraction", w.Type())
	}

	if err := w.Process(ctx, job); err != nil {
		t.Fatalf("processing: %v", err)
	}

	// Verify document is still processing (chunking job created, not yet complete).
	updatedDoc, err := docStore.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting document: %v", err)
	}
	if updatedDoc.Status != "processing" {
		t.Errorf("Status = %q, want processing", updatedDoc.Status)
	}

	// Verify extracted text.
	content, err := docStore.GetContent(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting content: %v", err)
	}
	if content.ExtractedText == "" {
		t.Error("expected non-empty extracted text")
	}

	// Verify a chunking job was created.
	jobs, err := jobStore.List(ctx, store.ListJobsOptions{DocumentID: doc.ID, Status: "queued"})
	if err != nil {
		t.Fatalf("listing jobs: %v", err)
	}
	found := false
	for _, j := range jobs {
		if j.JobType == "chunking" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a queued chunking job to be created")
	}
}

func TestExtractionWorker_PlainText_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	jobStore := store.NewJobStore(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("extraction-plain-%d", time.Now().UnixNano()),
		"Plain Text Test", "", "system:test", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "plain-test", "Plain Test", "reference", "pending", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	if err := docStore.SaveContent(ctx, doc.ID, []byte("Simple plain text content"), "text/plain"); err != nil {
		t.Fatalf("saving content: %v", err)
	}

	job, err := jobStore.Create(ctx, doc.ID, "extraction")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	w := NewExtractionWorker(docStore, jobStore)
	if err := w.Process(ctx, job); err != nil {
		t.Fatalf("processing: %v", err)
	}

	content, err := docStore.GetContent(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting content: %v", err)
	}
	if content.ExtractedText != "Simple plain text content" {
		t.Errorf("ExtractedText = %q, want %q", content.ExtractedText, "Simple plain text content")
	}
}

func TestExtractionWorker_UnsupportedType_Integration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()

	topicStore := store.NewTopicStore(pool)
	docStore := store.NewDocumentStore(pool)
	jobStore := store.NewJobStore(pool)

	topic, err := topicStore.Create(ctx, fmt.Sprintf("extraction-unsupported-%d", time.Now().UnixNano()),
		"Unsupported Test", "", "system:test", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating topic: %v", err)
	}
	t.Cleanup(func() { _ = topicStore.Delete(ctx, topic.ID) })

	doc, err := docStore.CreateWithStatus(ctx, topic.ID, "unsupported-test", "Unsupported Test", "reference", "pending", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating document: %v", err)
	}

	if err := docStore.SaveContent(ctx, doc.ID, []byte("binary data"), "application/pdf"); err != nil {
		t.Fatalf("saving content: %v", err)
	}

	job, err := jobStore.Create(ctx, doc.ID, "extraction")
	if err != nil {
		t.Fatalf("creating job: %v", err)
	}

	w := NewExtractionWorker(docStore, jobStore)
	err = w.Process(ctx, job)
	if err == nil {
		t.Fatal("expected error for unsupported content type")
	}

	// Document should be marked as failed.
	updatedDoc, err := docStore.Get(ctx, doc.ID)
	if err != nil {
		t.Fatalf("getting document: %v", err)
	}
	if updatedDoc.Status != "failed" {
		t.Errorf("Status = %q, want failed", updatedDoc.Status)
	}
}
