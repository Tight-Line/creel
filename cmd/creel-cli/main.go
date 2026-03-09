package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

var (
	endpoint string
	apiKey   string
	useTLS   bool
)

func main() {
	root := &cobra.Command{
		Use:   "creel-cli",
		Short: "Creel CLI client",
	}

	root.PersistentFlags().StringVar(&endpoint, "endpoint", envOr("CREEL_ENDPOINT", "127.0.0.1:8443"), "gRPC server endpoint")
	root.PersistentFlags().StringVar(&apiKey, "api-key", os.Getenv("CREEL_API_KEY"), "API key for authentication")
	root.PersistentFlags().BoolVar(&useTLS, "tls", false, "use TLS")

	root.AddCommand(healthCmd())
	root.AddCommand(adminCmd())
	root.AddCommand(topicCmd())
	root.AddCommand(configCmd())
	root.AddCommand(searchCmd())
	root.AddCommand(jobsCmd())
	root.AddCommand(uploadCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func dial() (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if useTLS {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return grpc.NewClient(endpoint, opts...)
}

func authCtx() context.Context {
	ctx := context.Background()
	if apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+apiKey)
	}
	return ctx
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// healthCmd returns the health check command.
func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check server health",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewAdminServiceClient(conn).Health(authCtx(), &pb.HealthRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
}

// adminCmd returns the admin command group.
func adminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "System account management",
	}

	createCmd := &cobra.Command{
		Use:   "create-account [name]",
		Short: "Create a system account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewAdminServiceClient(conn).CreateSystemAccount(authCtx(), &pb.CreateSystemAccountRequest{
				Name: args[0],
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list-accounts",
		Short: "List system accounts",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewAdminServiceClient(conn).ListSystemAccounts(authCtx(), &pb.ListSystemAccountsRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	var gracePeriod int32
	rotateCmd := &cobra.Command{
		Use:   "rotate-key [account-id]",
		Short: "Rotate API key for a system account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewAdminServiceClient(conn).RotateKey(authCtx(), &pb.RotateKeyRequest{
				AccountId:          args[0],
				GracePeriodSeconds: gracePeriod,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	rotateCmd.Flags().Int32Var(&gracePeriod, "grace-period", 0, "grace period in seconds")

	revokeCmd := &cobra.Command{
		Use:   "revoke-key [account-id]",
		Short: "Revoke API key for a system account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewAdminServiceClient(conn).RevokeKey(authCtx(), &pb.RevokeKeyRequest{
				AccountId: args[0],
			})
			if err != nil {
				return err
			}
			fmt.Println("key revoked")
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, rotateCmd, revokeCmd)
	return cmd
}

// topicCmd returns the topic command group.
func topicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topic",
		Short: "Topic management",
	}

	var createLLMConfig, createEmbConfig, createPromptConfig string
	createCmd := &cobra.Command{
		Use:   "create [slug] [name]",
		Short: "Create a topic",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			req := &pb.CreateTopicRequest{
				Slug: args[0],
				Name: args[1],
			}
			if createLLMConfig != "" {
				req.LlmConfigId = &createLLMConfig
			}
			if createEmbConfig != "" {
				req.EmbeddingConfigId = &createEmbConfig
			}
			if createPromptConfig != "" {
				req.ExtractionPromptConfigId = &createPromptConfig
			}

			resp, err := pb.NewTopicServiceClient(conn).CreateTopic(authCtx(), req)
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createLLMConfig, "llm-config", "", "LLM config ID")
	createCmd.Flags().StringVar(&createEmbConfig, "embedding-config", "", "embedding config ID")
	createCmd.Flags().StringVar(&createPromptConfig, "prompt-config", "", "extraction prompt config ID")

	var updateName, updateDescription, updateLLMConfig, updateEmbConfig, updatePromptConfig string
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			req := &pb.UpdateTopicRequest{
				Id:          args[0],
				Name:        updateName,
				Description: updateDescription,
			}
			if updateLLMConfig != "" {
				req.LlmConfigId = &updateLLMConfig
			}
			if updateEmbConfig != "" {
				req.EmbeddingConfigId = &updateEmbConfig
			}
			if updatePromptConfig != "" {
				req.ExtractionPromptConfigId = &updatePromptConfig
			}

			resp, err := pb.NewTopicServiceClient(conn).UpdateTopic(authCtx(), req)
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "new name")
	updateCmd.Flags().StringVar(&updateDescription, "description", "", "new description")
	updateCmd.Flags().StringVar(&updateLLMConfig, "llm-config", "", "LLM config ID")
	updateCmd.Flags().StringVar(&updateEmbConfig, "embedding-config", "", "embedding config ID")
	updateCmd.Flags().StringVar(&updatePromptConfig, "prompt-config", "", "extraction prompt config ID")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List accessible topics",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewTopicServiceClient(conn).ListTopics(authCtx(), &pb.ListTopicsRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	var permission string
	grantCmd := &cobra.Command{
		Use:   "grant [topic-id] [principal]",
		Short: "Grant access to a topic",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			var perm pb.Permission
			switch permission {
			case "read":
				perm = pb.Permission_PERMISSION_READ
			case "write":
				perm = pb.Permission_PERMISSION_WRITE
			case "admin":
				perm = pb.Permission_PERMISSION_ADMIN
			default:
				return fmt.Errorf("invalid permission: %s (must be read, write, or admin)", permission)
			}

			resp, err := pb.NewTopicServiceClient(conn).GrantAccess(authCtx(), &pb.GrantAccessRequest{
				TopicId:    args[0],
				Principal:  args[1],
				Permission: perm,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	grantCmd.Flags().StringVar(&permission, "permission", "read", "permission level (read, write, admin)")

	grantsCmd := &cobra.Command{
		Use:   "grants [topic-id]",
		Short: "List grants for a topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewTopicServiceClient(conn).ListGrants(authCtx(), &pb.ListGrantsRequest{
				TopicId: args[0],
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.AddCommand(createCmd, updateCmd, listCmd, grantCmd, grantsCmd)
	return cmd
}

// searchCmd returns the search command.
func searchCmd() *cobra.Command {
	var topicIDs []string
	var topK int32

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for chunks (requires embedding input via stdin)",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			// Read embedding from stdin as JSON array.
			var embedding []float64
			dec := json.NewDecoder(os.Stdin)
			if err := dec.Decode(&embedding); err != nil {
				return fmt.Errorf("reading embedding from stdin: %w", err)
			}

			resp, err := pb.NewRetrievalServiceClient(conn).Search(authCtx(), &pb.SearchRequest{
				TopicIds:       topicIDs,
				QueryEmbedding: embedding,
				TopK:           topK,
			})
			if err != nil {
				return err
			}

			// Build output with citation info when present.
			type citationOutput struct {
				URL         string `json:"url,omitempty"`
				Author      string `json:"author,omitempty"`
				PublishedAt string `json:"published_at,omitempty"`
			}
			type resultOutput struct {
				ChunkID    string          `json:"chunk_id"`
				DocumentID string          `json:"document_id"`
				TopicID    string          `json:"topic_id"`
				Score      float64         `json:"score"`
				Content    string          `json:"content"`
				Citation   *citationOutput `json:"citation,omitempty"`
			}
			var output []resultOutput
			for _, r := range resp.GetResults() {
				out := resultOutput{
					DocumentID: r.GetDocumentId(),
					TopicID:    r.GetTopicId(),
					Score:      r.GetScore(),
				}
				if r.GetChunk() != nil {
					out.ChunkID = r.GetChunk().GetId()
					out.Content = r.GetChunk().GetContent()
				}
				if dc := r.GetDocumentCitation(); dc != nil {
					c := &citationOutput{
						URL:    dc.GetUrl(),
						Author: dc.GetAuthor(),
					}
					if dc.GetPublishedAt() != nil {
						c.PublishedAt = dc.GetPublishedAt().AsTime().Format("2006-01-02")
					}
					if c.URL != "" || c.Author != "" || c.PublishedAt != "" {
						out.Citation = c
					}
				}
				output = append(output, out)
			}
			printJSON(output)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&topicIDs, "topic-ids", nil, "topic IDs to search (empty = all accessible)")
	cmd.Flags().Int32Var(&topK, "top-k", 10, "number of results")
	return cmd
}
