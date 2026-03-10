package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	pb "github.com/Tight-Line/creel/gen/creel/v1"
)

// configCmd returns the config command group.
func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management (API keys, LLM, embedding, extraction prompts)",
	}

	cmd.AddCommand(apiKeyCmd(), llmCmd(), embeddingCmd(), promptCmd(), vectorBackendCmd())
	return cmd
}

// ---------------------------------------------------------------------------
// API Key Config
// ---------------------------------------------------------------------------

func apiKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "API key configuration management",
	}

	// create
	var createName, createProvider, createAPIKey string
	var createDefault bool
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an API key config",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).CreateAPIKeyConfig(authCtx(), &pb.CreateAPIKeyConfigRequest{
				Name:      createName,
				Provider:  createProvider,
				ApiKey:    createAPIKey,
				IsDefault: createDefault,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "config name (required)")
	createCmd.Flags().StringVar(&createProvider, "provider", "", "provider (e.g. openai, anthropic) (required)")
	createCmd.Flags().StringVar(&createAPIKey, "api-key", "", "API key value (required)")
	createCmd.Flags().BoolVar(&createDefault, "default", false, "set as default")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("provider")
	_ = createCmd.MarkFlagRequired("api-key")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List API key configs",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).ListAPIKeyConfigs(authCtx(), &pb.ListAPIKeyConfigsRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// get
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get an API key config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).GetAPIKeyConfig(authCtx(), &pb.GetAPIKeyConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// update
	var updateName, updateProvider, updateAPIKey string
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update an API key config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).UpdateAPIKeyConfig(authCtx(), &pb.UpdateAPIKeyConfigRequest{
				Id:       args[0],
				Name:     updateName,
				Provider: updateProvider,
				ApiKey:   updateAPIKey,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "new name")
	updateCmd.Flags().StringVar(&updateProvider, "provider", "", "new provider")
	updateCmd.Flags().StringVar(&updateAPIKey, "api-key", "", "new API key value")

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete an API key config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewConfigServiceClient(conn).DeleteAPIKeyConfig(authCtx(), &pb.DeleteAPIKeyConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			fmt.Println("deleted")
			return nil
		},
	}

	// set-default
	setDefaultCmd := &cobra.Command{
		Use:   "set-default [id]",
		Short: "Set an API key config as default",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).SetDefaultAPIKeyConfig(authCtx(), &pb.SetDefaultAPIKeyConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, getCmd, updateCmd, deleteCmd, setDefaultCmd)
	return cmd
}

// ---------------------------------------------------------------------------
// LLM Config
// ---------------------------------------------------------------------------

func llmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llm",
		Short: "LLM configuration management",
	}

	// create
	var createName, createProvider, createModel, createAPIKeyConfigID string
	var createParams []string
	var createDefault bool
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an LLM config",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			params, err := parseParams(createParams)
			if err != nil {
				return err
			}

			resp, err := pb.NewConfigServiceClient(conn).CreateLLMConfig(authCtx(), &pb.CreateLLMConfigRequest{
				Name:           createName,
				Provider:       createProvider,
				Model:          createModel,
				ApiKeyConfigId: createAPIKeyConfigID,
				Parameters:     params,
				IsDefault:      createDefault,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "config name (required)")
	createCmd.Flags().StringVar(&createProvider, "provider", "", "provider (required)")
	createCmd.Flags().StringVar(&createModel, "model", "", "model name (required)")
	createCmd.Flags().StringVar(&createAPIKeyConfigID, "apikey-config", "", "API key config ID (required)")
	createCmd.Flags().StringSliceVar(&createParams, "param", nil, "parameter key=value (repeatable)")
	createCmd.Flags().BoolVar(&createDefault, "default", false, "set as default")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("provider")
	_ = createCmd.MarkFlagRequired("model")
	_ = createCmd.MarkFlagRequired("apikey-config")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List LLM configs",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).ListLLMConfigs(authCtx(), &pb.ListLLMConfigsRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// get
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get an LLM config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).GetLLMConfig(authCtx(), &pb.GetLLMConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// update
	var updateName, updateProvider, updateModel, updateAPIKeyConfigID string
	var updateParams []string
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update an LLM config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			params, err := parseParams(updateParams)
			if err != nil {
				return err
			}

			resp, err := pb.NewConfigServiceClient(conn).UpdateLLMConfig(authCtx(), &pb.UpdateLLMConfigRequest{
				Id:             args[0],
				Name:           updateName,
				Provider:       updateProvider,
				Model:          updateModel,
				ApiKeyConfigId: updateAPIKeyConfigID,
				Parameters:     params,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "new name")
	updateCmd.Flags().StringVar(&updateProvider, "provider", "", "new provider")
	updateCmd.Flags().StringVar(&updateModel, "model", "", "new model")
	updateCmd.Flags().StringVar(&updateAPIKeyConfigID, "apikey-config", "", "new API key config ID")
	updateCmd.Flags().StringSliceVar(&updateParams, "param", nil, "parameter key=value (repeatable)")

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete an LLM config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewConfigServiceClient(conn).DeleteLLMConfig(authCtx(), &pb.DeleteLLMConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			fmt.Println("deleted")
			return nil
		},
	}

	// set-default
	setDefaultCmd := &cobra.Command{
		Use:   "set-default [id]",
		Short: "Set an LLM config as default",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).SetDefaultLLMConfig(authCtx(), &pb.SetDefaultLLMConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, getCmd, updateCmd, deleteCmd, setDefaultCmd)
	return cmd
}

// ---------------------------------------------------------------------------
// Embedding Config
// ---------------------------------------------------------------------------

func embeddingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "embedding",
		Short: "Embedding configuration management",
	}

	// create
	var createName, createProvider, createModel, createAPIKeyConfigID string
	var createDimensions int32
	var createDefault bool
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an embedding config",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).CreateEmbeddingConfig(authCtx(), &pb.CreateEmbeddingConfigRequest{
				Name:           createName,
				Provider:       createProvider,
				Model:          createModel,
				Dimensions:     createDimensions,
				ApiKeyConfigId: createAPIKeyConfigID,
				IsDefault:      createDefault,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "config name (required)")
	createCmd.Flags().StringVar(&createProvider, "provider", "", "provider (required)")
	createCmd.Flags().StringVar(&createModel, "model", "", "model name (required)")
	createCmd.Flags().Int32Var(&createDimensions, "dimensions", 0, "embedding dimensions (required)")
	createCmd.Flags().StringVar(&createAPIKeyConfigID, "apikey-config", "", "API key config ID (required)")
	createCmd.Flags().BoolVar(&createDefault, "default", false, "set as default")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("provider")
	_ = createCmd.MarkFlagRequired("model")
	_ = createCmd.MarkFlagRequired("dimensions")
	_ = createCmd.MarkFlagRequired("apikey-config")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List embedding configs",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).ListEmbeddingConfigs(authCtx(), &pb.ListEmbeddingConfigsRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// get
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get an embedding config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).GetEmbeddingConfig(authCtx(), &pb.GetEmbeddingConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// update (only name and apikey-config can be changed)
	var updateName, updateAPIKeyConfigID string
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update an embedding config (name and API key config only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).UpdateEmbeddingConfig(authCtx(), &pb.UpdateEmbeddingConfigRequest{
				Id:             args[0],
				Name:           updateName,
				ApiKeyConfigId: updateAPIKeyConfigID,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "new name")
	updateCmd.Flags().StringVar(&updateAPIKeyConfigID, "apikey-config", "", "new API key config ID")

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete an embedding config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewConfigServiceClient(conn).DeleteEmbeddingConfig(authCtx(), &pb.DeleteEmbeddingConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			fmt.Println("deleted")
			return nil
		},
	}

	// set-default
	setDefaultCmd := &cobra.Command{
		Use:   "set-default [id]",
		Short: "Set an embedding config as default",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).SetDefaultEmbeddingConfig(authCtx(), &pb.SetDefaultEmbeddingConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, getCmd, updateCmd, deleteCmd, setDefaultCmd)
	return cmd
}

// ---------------------------------------------------------------------------
// Extraction Prompt Config
// ---------------------------------------------------------------------------

func promptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Extraction prompt configuration management",
	}

	// create
	var createName, createPrompt, createDescription string
	var createDefault bool
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an extraction prompt config",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).CreateExtractionPromptConfig(authCtx(), &pb.CreateExtractionPromptConfigRequest{
				Name:        createName,
				Prompt:      createPrompt,
				Description: createDescription,
				IsDefault:   createDefault,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "config name (required)")
	createCmd.Flags().StringVar(&createPrompt, "prompt", "", "extraction prompt text (required)")
	createCmd.Flags().StringVar(&createDescription, "description", "", "description")
	createCmd.Flags().BoolVar(&createDefault, "default", false, "set as default")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("prompt")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List extraction prompt configs",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).ListExtractionPromptConfigs(authCtx(), &pb.ListExtractionPromptConfigsRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// get
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get an extraction prompt config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).GetExtractionPromptConfig(authCtx(), &pb.GetExtractionPromptConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// update
	var updateName, updatePrompt, updateDescription string
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update an extraction prompt config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).UpdateExtractionPromptConfig(authCtx(), &pb.UpdateExtractionPromptConfigRequest{
				Id:          args[0],
				Name:        updateName,
				Prompt:      updatePrompt,
				Description: updateDescription,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "new name")
	updateCmd.Flags().StringVar(&updatePrompt, "prompt", "", "new prompt text")
	updateCmd.Flags().StringVar(&updateDescription, "description", "", "new description")

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete an extraction prompt config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewConfigServiceClient(conn).DeleteExtractionPromptConfig(authCtx(), &pb.DeleteExtractionPromptConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			fmt.Println("deleted")
			return nil
		},
	}

	// set-default
	setDefaultCmd := &cobra.Command{
		Use:   "set-default [id]",
		Short: "Set an extraction prompt config as default",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).SetDefaultExtractionPromptConfig(authCtx(), &pb.SetDefaultExtractionPromptConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, getCmd, updateCmd, deleteCmd, setDefaultCmd)
	return cmd
}

// ---------------------------------------------------------------------------
// Vector Backend Config
// ---------------------------------------------------------------------------

func vectorBackendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vector-backend",
		Short: "Vector backend configuration management",
	}

	// create
	var createName, createBackend string
	var createConfig []string
	var createDefault bool
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a vector backend config",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			config, err := parseParams(createConfig)
			if err != nil {
				return err
			}

			resp, err := pb.NewConfigServiceClient(conn).CreateVectorBackendConfig(authCtx(), &pb.CreateVectorBackendConfigRequest{
				Name:      createName,
				Backend:   createBackend,
				Config:    config,
				IsDefault: createDefault,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "config name (required)")
	createCmd.Flags().StringVar(&createBackend, "backend", "", "backend type, e.g. pgvector (required)")
	createCmd.Flags().StringSliceVar(&createConfig, "config", nil, "config key=value (repeatable)")
	createCmd.Flags().BoolVar(&createDefault, "default", false, "set as default")
	_ = createCmd.MarkFlagRequired("name")
	_ = createCmd.MarkFlagRequired("backend")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List vector backend configs",
		RunE: func(_ *cobra.Command, _ []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).ListVectorBackendConfigs(authCtx(), &pb.ListVectorBackendConfigsRequest{})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// get
	getCmd := &cobra.Command{
		Use:   "get [id]",
		Short: "Get a vector backend config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).GetVectorBackendConfig(authCtx(), &pb.GetVectorBackendConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	// update
	var updateName string
	var updateConfig []string
	updateCmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Update a vector backend config (name and config only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			config, err := parseParams(updateConfig)
			if err != nil {
				return err
			}

			resp, err := pb.NewConfigServiceClient(conn).UpdateVectorBackendConfig(authCtx(), &pb.UpdateVectorBackendConfigRequest{
				Id:     args[0],
				Name:   updateName,
				Config: config,
			})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
	updateCmd.Flags().StringVar(&updateName, "name", "", "new name")
	updateCmd.Flags().StringSliceVar(&updateConfig, "config", nil, "config key=value (repeatable)")

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a vector backend config",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			_, err = pb.NewConfigServiceClient(conn).DeleteVectorBackendConfig(authCtx(), &pb.DeleteVectorBackendConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			fmt.Println("deleted")
			return nil
		},
	}

	// set-default
	setDefaultCmd := &cobra.Command{
		Use:   "set-default [id]",
		Short: "Set a vector backend config as default",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			conn, err := dial()
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			resp, err := pb.NewConfigServiceClient(conn).SetDefaultVectorBackendConfig(authCtx(), &pb.SetDefaultVectorBackendConfigRequest{Id: args[0]})
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, getCmd, updateCmd, deleteCmd, setDefaultCmd)
	return cmd
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseParams converts ["key=value", ...] to map[string]string.
func parseParams(params []string) (map[string]string, error) {
	if len(params) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(params))
	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("invalid parameter %q (expected key=value)", p)
		}
		m[k] = v
	}
	return m, nil
}
