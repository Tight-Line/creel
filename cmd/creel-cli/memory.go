package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// memoryCmd returns the memory command group.
func memoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Memory management",
	}

	var scope string
	var includeAll bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List memories in a scope",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewMemoryServiceClient(conn).ListMemories(authCtx(), &pb.ListMemoriesRequest{
				Scope:              scope,
				IncludeInvalidated: includeAll,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	listCmd.Flags().StringVar(&scope, "scope", "default", "memory scope")
	listCmd.Flags().BoolVar(&includeAll, "all", false, "include invalidated memories")

	var addScope, content string
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a memory to a scope",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewMemoryServiceClient(conn).AddMemory(authCtx(), &pb.AddMemoryRequest{
				Scope:   addScope,
				Content: content,
			})
			if err != nil {
				return fmt.Errorf("adding memory: %w", err)
			}
			printJSON(resp)
			return nil
		},
	}
	addCmd.Flags().StringVar(&addScope, "scope", "default", "memory scope")
	addCmd.Flags().StringVar(&content, "content", "", "memory content (required)")
	_ = addCmd.MarkFlagRequired("content")

	deleteCmd := &cobra.Command{
		Use:   "delete [memory-id]",
		Short: "Delete (invalidate) a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewMemoryServiceClient(conn).DeleteMemory(authCtx(), &pb.DeleteMemoryRequest{
				Id: args[0],
			})
			if err != nil {
				return fmt.Errorf("deleting memory: %w", err)
			}
			fmt.Println("memory deleted")
			return nil
		},
	}

	scopesCmd := &cobra.Command{
		Use:   "scopes",
		Short: "List all memory scopes",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewMemoryServiceClient(conn).ListScopes(authCtx(), &pb.ListScopesRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	var searchScope, queryText string
	var topK int32
	searchCmd := &cobra.Command{
		Use:   "search",
		Short: "Search memories by embedding or text query",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			req := &pb.SearchMemoriesRequest{
				Scope:     searchScope,
				QueryText: queryText,
				TopK:      topK,
			}

			// If no query text, try reading embedding from stdin.
			if queryText == "" {
				var embedding []float64
				dec := json.NewDecoder(os.Stdin)
				if err := dec.Decode(&embedding); err != nil {
					return fmt.Errorf("reading embedding from stdin (or provide --query): %w", err)
				}
				req.QueryEmbedding = embedding
			}

			resp, err := pb.NewMemoryServiceClient(conn).SearchMemories(authCtx(), req)
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	searchCmd.Flags().StringVar(&searchScope, "scope", "default", "memory scope")
	searchCmd.Flags().StringVar(&queryText, "query", "", "text query (requires embedding provider)")
	searchCmd.Flags().Int32Var(&topK, "top-k", 10, "number of results")

	cmd.AddCommand(listCmd, addCmd, deleteCmd, scopesCmd, searchCmd)
	return cmd
}
