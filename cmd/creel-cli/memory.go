package main

import (
	"fmt"

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

	var getScopes []string
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get active memories, optionally filtered by scopes",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewMemoryServiceClient(conn).GetMemories(authCtx(), &pb.GetMemoriesRequest{
				Scopes: getScopes,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	getCmd.Flags().StringSliceVar(&getScopes, "scope", nil, "memory scope(s) to filter by (repeatable)")

	cmd.AddCommand(listCmd, addCmd, deleteCmd, scopesCmd, getCmd)
	return cmd
}
