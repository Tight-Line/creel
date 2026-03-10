package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

func linkServerWithMock(db *mockDBTX, authz *mockAuthorizer) *LinkServer {
	return NewLinkServer(
		store.NewLinkStore(db),
		store.NewChunkStore(db),
		authz,
	)
}

func linkAuthedCtx() context.Context {
	return auth.ContextWithPrincipal(context.Background(), &auth.Principal{ID: "user:test"})
}

// --- CreateLink ---

func TestLinkServer_CreateLink_Unauthenticated(t *testing.T) {
	srv := linkServerWithMock(&mockDBTX{}, &mockAuthorizer{})
	_, err := srv.CreateLink(context.Background(), &pb.CreateLinkRequest{
		SourceChunkId: "s", TargetChunkId: "t",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestLinkServer_CreateLink_MissingFields(t *testing.T) {
	srv := linkServerWithMock(&mockDBTX{}, &mockAuthorizer{})
	_, err := srv.CreateLink(linkAuthedCtx(), &pb.CreateLinkRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestLinkServer_CreateLink_SourceNotFound(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("chunk not found")}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{})
	_, err := srv.CreateLink(linkAuthedCtx(), &pb.CreateLinkRequest{
		SourceChunkId: "bad", TargetChunkId: "t",
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestLinkServer_CreateLink_PermissionDenied(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				return &linkChunkRow{id: "c1", docID: "d1"}
			}
			return &linkTopicIDRow{topicID: "t1"}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{checkErr: errors.New("denied")})
	_, err := srv.CreateLink(linkAuthedCtx(), &pb.CreateLinkRequest{
		SourceChunkId: "c1", TargetChunkId: "c2",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

func TestLinkServer_CreateLink_TargetNotFound(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			switch callCount {
			case 1:
				return &linkChunkRow{id: "c1", docID: "d1"}
			case 2:
				return &linkTopicIDRow{topicID: "t1"}
			default:
				return &mockRow{err: errors.New("chunk not found")}
			}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{})
	_, err := srv.CreateLink(linkAuthedCtx(), &pb.CreateLinkRequest{
		SourceChunkId: "c1", TargetChunkId: "bad",
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestLinkServer_CreateLink_StoreError(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			switch callCount {
			case 1:
				return &linkChunkRow{id: "c1", docID: "d1"}
			case 2:
				return &linkTopicIDRow{topicID: "t1"}
			case 3:
				return &linkChunkRow{id: "c2", docID: "d2"}
			default:
				return &mockRow{err: errors.New("db error")}
			}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{})
	_, err := srv.CreateLink(linkAuthedCtx(), &pb.CreateLinkRequest{
		SourceChunkId: "c1", TargetChunkId: "c2",
	})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// --- DeleteLink ---

func TestLinkServer_DeleteLink_Unauthenticated(t *testing.T) {
	srv := linkServerWithMock(&mockDBTX{}, &mockAuthorizer{})
	_, err := srv.DeleteLink(context.Background(), &pb.DeleteLinkRequest{Id: "x"})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestLinkServer_DeleteLink_MissingID(t *testing.T) {
	srv := linkServerWithMock(&mockDBTX{}, &mockAuthorizer{})
	_, err := srv.DeleteLink(linkAuthedCtx(), &pb.DeleteLinkRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestLinkServer_DeleteLink_NotFound(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("link not found")}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{})
	_, err := srv.DeleteLink(linkAuthedCtx(), &pb.DeleteLinkRequest{Id: "bad"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestLinkServer_DeleteLink_PermissionDenied(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			switch callCount {
			case 1:
				return &linkLinkRow{id: "l1", sourceChunk: "c1", targetChunk: "c2"}
			case 2:
				return &linkChunkRow{id: "c1", docID: "d1"}
			default:
				return &linkTopicIDRow{topicID: "t1"}
			}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{checkErr: errors.New("denied")})
	_, err := srv.DeleteLink(linkAuthedCtx(), &pb.DeleteLinkRequest{Id: "l1"})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

// --- ListLinks ---

func TestLinkServer_ListLinks_Unauthenticated(t *testing.T) {
	srv := linkServerWithMock(&mockDBTX{}, &mockAuthorizer{})
	_, err := srv.ListLinks(context.Background(), &pb.ListLinksRequest{ChunkId: "x"})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestLinkServer_ListLinks_MissingChunkID(t *testing.T) {
	srv := linkServerWithMock(&mockDBTX{}, &mockAuthorizer{})
	_, err := srv.ListLinks(linkAuthedCtx(), &pb.ListLinksRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestLinkServer_ListLinks_ChunkNotFound(t *testing.T) {
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{err: errors.New("chunk not found")}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{})
	_, err := srv.ListLinks(linkAuthedCtx(), &pb.ListLinksRequest{ChunkId: "bad"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestLinkServer_ListLinks_PermissionDenied(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				return &linkChunkRow{id: "c1", docID: "d1"}
			}
			return &linkTopicIDRow{topicID: "t1"}
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{checkErr: errors.New("denied")})
	_, err := srv.ListLinks(linkAuthedCtx(), &pb.ListLinksRequest{ChunkId: "c1"})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}
}

func TestLinkServer_ListLinks_QueryError(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				return &linkChunkRow{id: "c1", docID: "d1"}
			}
			return &linkTopicIDRow{topicID: "t1"}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{})
	_, err := srv.ListLinks(linkAuthedCtx(), &pb.ListLinksRequest{ChunkId: "c1"})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestLinkServer_ListLinks_Empty(t *testing.T) {
	callCount := 0
	db := &mockDBTX{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			if callCount == 1 {
				return &linkChunkRow{id: "c1", docID: "d1"}
			}
			return &linkTopicIDRow{topicID: "t1"}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &linkEmptyRows{}, nil
		},
	}
	srv := linkServerWithMock(db, &mockAuthorizer{})
	resp, err := srv.ListLinks(linkAuthedCtx(), &pb.ListLinksRequest{ChunkId: "c1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetLinks()) != 0 {
		t.Errorf("expected 0 links, got %d", len(resp.GetLinks()))
	}
}

// --- Proto conversion ---

func TestProtoLinkTypeConversions(t *testing.T) {
	tests := []struct {
		protoType pb.LinkType
		storeType string
	}{
		{pb.LinkType_LINK_TYPE_MANUAL, "manual"},
		{pb.LinkType_LINK_TYPE_AUTO, "auto"},
		{pb.LinkType_LINK_TYPE_COMPACTION_TRANSFER, "compaction_transfer"},
		{pb.LinkType_LINK_TYPE_UNSPECIFIED, "manual"},
	}

	for _, tt := range tests {
		got := protoLinkTypeToStore(tt.protoType)
		if got != tt.storeType {
			t.Errorf("protoLinkTypeToStore(%v) = %q, want %q", tt.protoType, got, tt.storeType)
		}
	}

	reverseTests := []struct {
		storeType string
		protoType pb.LinkType
	}{
		{"manual", pb.LinkType_LINK_TYPE_MANUAL},
		{"auto", pb.LinkType_LINK_TYPE_AUTO},
		{"compaction_transfer", pb.LinkType_LINK_TYPE_COMPACTION_TRANSFER},
		{"unknown", pb.LinkType_LINK_TYPE_UNSPECIFIED},
	}

	for _, tt := range reverseTests {
		got := storeLinkTypeToProto(tt.storeType)
		if got != tt.protoType {
			t.Errorf("storeLinkTypeToProto(%q) = %v, want %v", tt.storeType, got, tt.protoType)
		}
	}
}

// --- Mock helpers specific to link tests ---

// linkChunkRow returns chunk data for ChunkStore.Get queries.
type linkChunkRow struct {
	id, docID string
}

func (r *linkChunkRow) Scan(dest ...any) error {
	// ChunkStore.Get: id, document_id, sequence, content, embedding_id, status, compacted_by, metadata, created_at
	*(dest[0].(*string)) = r.id
	*(dest[1].(*string)) = r.docID
	*(dest[2].(*int)) = 1
	*(dest[3].(*string)) = "content"
	// dest[4] *string (embedding_id) - leave nil
	*(dest[5].(*string)) = "active"
	// dest[6] *string (compacted_by) - leave nil
	*(dest[7].(*[]byte)) = []byte("{}")
	*(dest[8].(*time.Time)) = time.Now()
	return nil
}

// linkTopicIDRow returns a topic ID for DocumentTopicID queries.
type linkTopicIDRow struct {
	topicID string
}

func (r *linkTopicIDRow) Scan(dest ...any) error {
	*(dest[0].(*string)) = r.topicID
	return nil
}

// linkLinkRow returns link data for LinkStore.Get queries.
type linkLinkRow struct {
	id, sourceChunk, targetChunk string
}

func (r *linkLinkRow) Scan(dest ...any) error {
	// LinkStore.Get: id, source_chunk, target_chunk, link_type, created_by, metadata, created_at
	*(dest[0].(*string)) = r.id
	*(dest[1].(*string)) = r.sourceChunk
	*(dest[2].(*string)) = r.targetChunk
	*(dest[3].(*string)) = "manual"
	*(dest[4].(*string)) = "user:test"
	*(dest[5].(*[]byte)) = []byte("{}")
	*(dest[6].(*time.Time)) = time.Now()
	return nil
}

// linkEmptyRows implements pgx.Rows with no data.
type linkEmptyRows struct{}

func (r *linkEmptyRows) Close()                                       {}
func (r *linkEmptyRows) Err() error                                   { return nil }
func (r *linkEmptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *linkEmptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *linkEmptyRows) Next() bool                                   { return false }
func (r *linkEmptyRows) Scan(_ ...any) error                          { return nil }
func (r *linkEmptyRows) Values() ([]any, error)                       { return nil, nil }
func (r *linkEmptyRows) RawValues() [][]byte                          { return nil }
func (r *linkEmptyRows) Conn() *pgx.Conn                              { return nil }
