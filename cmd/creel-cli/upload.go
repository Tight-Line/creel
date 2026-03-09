package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// uploadCmd returns the upload command.
func uploadCmd() *cobra.Command {
	var topicID, name, author, url, sourceURL, contentType, filePath string

	cmd := &cobra.Command{
		Use:   "upload --topic <id> --file <path>",
		Short: "Upload a document for processing",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			req := &pb.UploadDocumentRequest{
				TopicId:     topicID,
				Name:        name,
				Author:      author,
				Url:         url,
				SourceUrl:   sourceURL,
				ContentType: contentType,
			}

			if sourceURL == "" {
				if filePath == "" {
					return fmt.Errorf("either --file or --source-url is required")
				}
				data, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				req.File = data
				if name == "" {
					// Use filename as name if not provided.
					req.Name = filePath
				}
			}

			resp, err := pb.NewDocumentServiceClient(conn).UploadDocument(authCtx(), req)
			if err != nil {
				return fmt.Errorf("uploading document: %w", err)
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.Flags().StringVar(&topicID, "topic", "", "topic ID (required)")
	cmd.Flags().StringVar(&filePath, "file", "", "path to file to upload")
	cmd.Flags().StringVar(&name, "name", "", "document name")
	cmd.Flags().StringVar(&author, "author", "", "document author")
	cmd.Flags().StringVar(&url, "url", "", "citation URL")
	cmd.Flags().StringVar(&sourceURL, "source-url", "", "URL to fetch document from")
	cmd.Flags().StringVar(&contentType, "content-type", "", "MIME type hint")

	return cmd
}
