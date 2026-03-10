package main

import (
	"strings"

	"github.com/spf13/cobra"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// compactCmd returns the compact command group.
func compactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact",
		Short: "Chunk compaction management",
	}

	// compact run: enqueue a background compaction job.
	var runDocID, runChunkIDs string
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Request background compaction for a document",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			req := &pb.RequestCompactionRequest{
				DocumentId: runDocID,
			}
			if runChunkIDs != "" {
				req.ChunkIds = strings.Split(runChunkIDs, ",")
			}

			resp, err := pb.NewCompactionServiceClient(conn).RequestCompaction(authCtx(), req)
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	runCmd.Flags().StringVar(&runDocID, "document", "", "document ID")
	runCmd.Flags().StringVar(&runChunkIDs, "chunk-ids", "", "comma-separated chunk IDs (optional; empty = all active)")
	_ = runCmd.MarkFlagRequired("document")

	// compact manual: synchronous compaction with caller-supplied summary.
	var manDocID, manChunkIDs, manSummary string
	manualCmd := &cobra.Command{
		Use:   "manual",
		Short: "Perform manual compaction with a provided summary",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewCompactionServiceClient(conn).Compact(authCtx(), &pb.CompactRequest{
				DocumentId:     manDocID,
				ChunkIds:       strings.Split(manChunkIDs, ","),
				SummaryContent: manSummary,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	manualCmd.Flags().StringVar(&manDocID, "document", "", "document ID")
	manualCmd.Flags().StringVar(&manChunkIDs, "chunk-ids", "", "comma-separated chunk IDs to compact")
	manualCmd.Flags().StringVar(&manSummary, "summary", "", "summary content for the compacted chunk")
	_ = manualCmd.MarkFlagRequired("document")
	_ = manualCmd.MarkFlagRequired("chunk-ids")
	_ = manualCmd.MarkFlagRequired("summary")

	// compact undo: reverse a compaction.
	var undoChunkID string
	undoCmd := &cobra.Command{
		Use:   "undo",
		Short: "Reverse a compaction by restoring source chunks",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewCompactionServiceClient(conn).Uncompact(authCtx(), &pb.UncompactRequest{
				SummaryChunkId: undoChunkID,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	undoCmd.Flags().StringVar(&undoChunkID, "summary-chunk", "", "summary chunk ID to uncompact")
	_ = undoCmd.MarkFlagRequired("summary-chunk")

	// compact history: list compaction records for a document.
	var histDocID string
	histCmd := &cobra.Command{
		Use:   "history",
		Short: "List compaction history for a document",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewCompactionServiceClient(conn).GetCompactionHistory(authCtx(), &pb.GetCompactionHistoryRequest{
				DocumentId: histDocID,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	histCmd.Flags().StringVar(&histDocID, "document", "", "document ID")
	_ = histCmd.MarkFlagRequired("document")

	cmd.AddCommand(runCmd, manualCmd, undoCmd, histCmd)
	return cmd
}
