package main

import (
	"github.com/spf13/cobra"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// linkCmd returns the link command group.
func linkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link management",
	}

	var sourceChunkID, targetChunkID, linkType string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a link between two chunks",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			lt := pb.LinkType_LINK_TYPE_MANUAL
			switch linkType {
			case "auto":
				lt = pb.LinkType_LINK_TYPE_AUTO
			case "compaction_transfer":
				lt = pb.LinkType_LINK_TYPE_COMPACTION_TRANSFER
			}

			resp, err := pb.NewLinkServiceClient(conn).CreateLink(authCtx(), &pb.CreateLinkRequest{
				SourceChunkId: sourceChunkID,
				TargetChunkId: targetChunkID,
				LinkType:      lt,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	createCmd.Flags().StringVar(&sourceChunkID, "source", "", "source chunk ID")
	createCmd.Flags().StringVar(&targetChunkID, "target", "", "target chunk ID")
	createCmd.Flags().StringVar(&linkType, "type", "manual", "link type (manual, auto, compaction_transfer)")
	_ = createCmd.MarkFlagRequired("source")
	_ = createCmd.MarkFlagRequired("target")

	var deleteID string
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a link",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewLinkServiceClient(conn).DeleteLink(authCtx(), &pb.DeleteLinkRequest{
				Id: deleteID,
			})
			if err != nil {
				return err
			}
			printJSON(map[string]string{"status": "deleted"})
			return nil
		},
	}
	deleteCmd.Flags().StringVar(&deleteID, "id", "", "link ID")
	_ = deleteCmd.MarkFlagRequired("id")

	var listChunkID string
	var includeBacklinks bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List links for a chunk",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewLinkServiceClient(conn).ListLinks(authCtx(), &pb.ListLinksRequest{
				ChunkId:          listChunkID,
				IncludeBacklinks: includeBacklinks,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	listCmd.Flags().StringVar(&listChunkID, "chunk", "", "chunk ID")
	listCmd.Flags().BoolVar(&includeBacklinks, "backlinks", false, "include backlinks (where chunk is target)")
	_ = listCmd.MarkFlagRequired("chunk")

	cmd.AddCommand(createCmd, deleteCmd, listCmd)
	return cmd
}
