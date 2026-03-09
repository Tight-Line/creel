package main

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// documentCmd returns the document command group.
func documentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "document",
		Short: "Document management",
	}

	var listTopic string
	listCmd := &cobra.Command{
		Use:   "list --topic <topic>",
		Short: "List documents in a topic",
		RunE: func(_ *cobra.Command, _ []string) error {
			if listTopic == "" {
				return fmt.Errorf("--topic is required")
			}
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			topicID, err := resolveTopicID(conn, listTopic)
			if err != nil {
				return err
			}

			resp, err := pb.NewDocumentServiceClient(conn).ListDocuments(authCtx(), &pb.ListDocumentsRequest{
				TopicId: topicID,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	listCmd.Flags().StringVar(&listTopic, "topic", "", "topic ID or slug (required)")

	var getTopic string
	getCmd := &cobra.Command{
		Use:   "get [id-or-slug]",
		Short: "Get a document by ID or slug",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			docID := args[0]
			if !uuidPattern.MatchString(docID) {
				if getTopic == "" {
					return fmt.Errorf("--topic is required when using a document slug")
				}
				topicID, err := resolveTopicID(conn, getTopic)
				if err != nil {
					return err
				}
				docID, err = resolveDocumentID(conn, topicID, docID)
				if err != nil {
					return err
				}
			}

			resp, err := pb.NewDocumentServiceClient(conn).GetDocument(authCtx(), &pb.GetDocumentRequest{
				Id: docID,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	getCmd.Flags().StringVar(&getTopic, "topic", "", "topic ID or slug (required when using document slug)")

	var deleteTopic string
	deleteCmd := &cobra.Command{
		Use:   "delete [id-or-slug]",
		Short: "Delete a document",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			docID := args[0]
			if !uuidPattern.MatchString(docID) {
				if deleteTopic == "" {
					return fmt.Errorf("--topic is required when using a document slug")
				}
				topicID, err := resolveTopicID(conn, deleteTopic)
				if err != nil {
					return err
				}
				docID, err = resolveDocumentID(conn, topicID, docID)
				if err != nil {
					return err
				}
			}

			_, err = pb.NewDocumentServiceClient(conn).DeleteDocument(authCtx(), &pb.DeleteDocumentRequest{
				Id: docID,
			})
			if err != nil {
				return err
			}
			fmt.Println("document deleted")
			return nil
		},
	}
	deleteCmd.Flags().StringVar(&deleteTopic, "topic", "", "topic ID or slug (required when using document slug)")

	cmd.AddCommand(listCmd, getCmd, deleteCmd)
	return cmd
}
