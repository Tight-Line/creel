package worker

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tight-Line/creel/internal/llm"
)

// failingLLMProvider always returns an error.
type failingLLMProvider struct{}

func (p *failingLLMProvider) Complete(_ context.Context, _ []llm.Message) (*llm.Response, error) {
	return nil, errors.New("LLM error")
}

// mockJobWithProgressDBTX supports CreateWithProgress and CreateDocless.
type mockJobWithProgressDBTX struct{}

func (m *mockJobWithProgressDBTX) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 1"), nil
}

func (m *mockJobWithProgressDBTX) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not configured")
}

func (m *mockJobWithProgressDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockJobRow{}
}

func (m *mockJobWithProgressDBTX) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not configured")
}
