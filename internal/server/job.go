package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
	"github.com/Tight-Line/creel/internal/auth"
	"github.com/Tight-Line/creel/internal/store"
)

// JobServer implements the JobService gRPC service.
type JobServer struct {
	pb.UnimplementedJobServiceServer
	jobStore   *store.JobStore
	docStore   *store.DocumentStore
	authorizer auth.Authorizer
}

// NewJobServer creates a new job service.
func NewJobServer(jobStore *store.JobStore, docStore *store.DocumentStore, authorizer auth.Authorizer) *JobServer {
	return &JobServer{
		jobStore:   jobStore,
		docStore:   docStore,
		authorizer: authorizer,
	}
}

// GetJob retrieves a job by ID. Requires read access to the job's document's topic.
func (s *JobServer) GetJob(ctx context.Context, req *pb.GetJobRequest) (*pb.ProcessingJob, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	job, err := s.jobStore.Get(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "job not found")
	}

	// Documentless jobs (e.g. memory maintenance from AddMemory) authorize
	// by matching the principal in the job's progress data.
	if job.DocumentID == "" {
		if principal, _ := job.Progress["principal"].(string); principal != p.ID {
			return nil, status.Error(codes.PermissionDenied, "not owner of this job")
		}
		return storeJobToProto(job), nil
	}

	topicID, err := s.docStore.TopicIDForDocument(ctx, job.DocumentID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "document not found")
	}

	if err := s.authorizer.Check(ctx, p, topicID, auth.ActionRead); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	return storeJobToProto(job), nil
}

// ListJobs lists jobs with optional filters. If topic_id is specified, requires
// read access to that topic. If document_id is specified, resolves the topic
// and requires read access. If neither, returns jobs across all accessible topics.
func (s *JobServer) ListJobs(ctx context.Context, req *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	p := auth.PrincipalFromContext(ctx)
	if p == nil {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	opts := store.ListJobsOptions{
		Status:    req.GetStatus(),
		PageSize:  int(req.GetPageSize()),
		PageToken: req.GetPageToken(),
	}

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	var jobs []*store.ProcessingJob
	var err error

	switch {
	case req.GetTopicId() != "":
		if authErr := s.authorizer.Check(ctx, p, req.GetTopicId(), auth.ActionRead); authErr != nil {
			return nil, status.Error(codes.PermissionDenied, authErr.Error())
		}
		jobs, err = s.jobStore.ListByTopicID(ctx, req.GetTopicId(), opts)

	case req.GetDocumentId() != "":
		topicID, docErr := s.docStore.TopicIDForDocument(ctx, req.GetDocumentId())
		if docErr != nil {
			return nil, status.Error(codes.NotFound, "document not found")
		}
		if authErr := s.authorizer.Check(ctx, p, topicID, auth.ActionRead); authErr != nil {
			return nil, status.Error(codes.PermissionDenied, authErr.Error())
		}
		opts.DocumentID = req.GetDocumentId()
		jobs, err = s.jobStore.List(ctx, opts)

	default:
		topicIDs, accessErr := s.authorizer.AccessibleTopics(ctx, p, auth.ActionRead)
		if accessErr != nil {
			return nil, status.Errorf(codes.Internal, "checking accessible topics: %v", accessErr)
		}
		if len(topicIDs) == 0 {
			return &pb.ListJobsResponse{}, nil
		}
		// Collect jobs across all accessible topics in a single batch query per topic.
		// For simplicity, we query per topic but this is bounded by the number of
		// accessible topics, not the number of jobs.
		var allJobs []*store.ProcessingJob
		for _, tid := range topicIDs {
			topicJobs, topicErr := s.jobStore.ListByTopicID(ctx, tid, opts)
			if topicErr != nil {
				return nil, status.Errorf(codes.Internal, "listing jobs: %v", topicErr)
			}
			allJobs = append(allJobs, topicJobs...)
		}
		jobs = allJobs
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing jobs: %v", err)
	}

	var nextPageToken string
	if len(jobs) > pageSize {
		nextPageToken = jobs[pageSize-1].ID
		jobs = jobs[:pageSize]
	}

	pbJobs := make([]*pb.ProcessingJob, len(jobs))
	for i, j := range jobs {
		pbJobs[i] = storeJobToProto(j)
	}

	return &pb.ListJobsResponse{
		Jobs:          pbJobs,
		NextPageToken: nextPageToken,
	}, nil
}

func storeJobToProto(j *store.ProcessingJob) *pb.ProcessingJob {
	pj := &pb.ProcessingJob{
		Id:         j.ID,
		DocumentId: j.DocumentID,
		JobType:    j.JobType,
		Status:     j.Status,
		Progress:   mapToStruct(j.Progress),
		CreatedAt:  timestamppb.New(j.CreatedAt),
	}
	if j.Error != nil {
		pj.Error = *j.Error
	}
	if j.StartedAt != nil {
		pj.StartedAt = timestamppb.New(*j.StartedAt)
	}
	if j.CompletedAt != nil {
		pj.CompletedAt = timestamppb.New(*j.CompletedAt)
	}
	return pj
}
