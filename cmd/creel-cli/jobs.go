package main

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// jobsCmd returns the jobs command group.
func jobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Processing job management",
	}

	var topicID, statusFilter string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List processing jobs",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resolvedTopicID := topicID
			if topicID != "" {
				var err error
				resolvedTopicID, err = resolveTopicID(conn, topicID)
				if err != nil {
					return err
				}
			}

			req := &pb.ListJobsRequest{
				TopicId: resolvedTopicID,
				Status:  statusFilter,
			}

			resp, err := pb.NewJobServiceClient(conn).ListJobs(authCtx(), req)
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	listCmd.Flags().StringVar(&topicID, "topic", "", "filter by topic ID or slug")
	listCmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (queued, running, completed, failed)")

	statusCmd := &cobra.Command{
		Use:   "status [job-id]",
		Short: "Get processing job status",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewJobServiceClient(conn).GetJob(authCtx(), &pb.GetJobRequest{
				Id: args[0],
			})
			if err != nil {
				return fmt.Errorf("getting job: %w", err)
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.AddCommand(listCmd, statusCmd)
	return cmd
}
