package main

import (
	"fmt"
	"os"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/client"
	"github.com/spf13/cobra"
)

func main() {
	cfgPath, _ := client.DefaultPath()
	cfg, _ := client.Load(cfgPath)

	root := &cobra.Command{
		Use:   "dfs",
		Short: "NimbusFS distributed file storage client",
	}

	serverFlag := root.PersistentFlags().String("server", "", "master REST address")
	root.PersistentFlags().StringVar(&cfgPath, "config", cfgPath, "config file path")

	root.AddCommand(
		uploadCmd(cfg, serverFlag),
		downloadCmd(cfg, serverFlag),
		deleteCmd(cfg, serverFlag),
		listCmd(cfg, serverFlag),
		statusCmd(cfg, serverFlag),
		loginCmd(cfg, cfgPath),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func apiFromFlags(cfg *client.Config, serverFlag *string) *client.API {
	server := client.ResolveServer(*serverFlag, cfg)
	return client.NewAPI(server, cfg.APIKey, cfg.Token)
}

func uploadCmd(cfg *client.Config, serverFlag *string) *cobra.Command {
	return &cobra.Command{
		Use:   "upload <filepath>",
		Short: "Upload a file to the cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := apiFromFlags(cfg, serverFlag)
			info, err := os.Stat(args[0])
			if err != nil {
				return err
			}
			progress := client.NewProgressWriter("uploading", info.Size())
			resp, err := api.Upload(args[0], progress)
			progress.Done()
			if err != nil {
				return err
			}
			fmt.Printf("uploaded %s as %s (%d chunks)\n", resp.FileName, resp.FileID, resp.Chunks)
			return nil
		},
	}
}

func downloadCmd(cfg *client.Config, serverFlag *string) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "download <fileId>",
		Short: "Download a file by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api := apiFromFlags(cfg, serverFlag)
			progress := client.NewProgressWriter("downloading", 0)
			if err := api.Download(args[0], output, progress); err != nil {
				return err
			}
			progress.Done()
			fmt.Printf("saved to %s/%s\n", output, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", ".", "output directory")
	return cmd
}

func deleteCmd(cfg *client.Config, serverFlag *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <fileId>",
		Short: "Delete a file by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiFromFlags(cfg, serverFlag).Delete(args[0])
		},
	}
}

func listCmd(cfg *client.Config, serverFlag *string) *cobra.Command {
	var page, limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files in the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiFromFlags(cfg, serverFlag).List(page, limit)
			if err != nil {
				return err
			}
			fmt.Printf("page %d — %d of %d files\n", resp.Page, len(resp.Files), resp.Total)
			for _, f := range resp.Files {
				fmt.Println(f)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "page number")
	cmd.Flags().IntVar(&limit, "limit", 20, "page size")
	return cmd
}

func statusCmd(cfg *client.Config, serverFlag *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cluster node health",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiFromFlags(cfg, serverFlag).Status()
			if err != nil {
				return err
			}
			fmt.Printf("nodes alive: %d  replication_factor: %d\n", resp.AliveNodes, resp.ReplicationFactor)
			fmt.Printf("storage used/total: %d / %d bytes\n", resp.UsedStorage, resp.TotalStorage)
			for _, n := range resp.Nodes {
				fmt.Printf("  %-20s status=%v used=%v\n", n["NodeID"], n["Status"], n["UsedSpace"])
			}
			return nil
		},
	}
}

func loginCmd(cfg *client.Config, cfgPath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store API key in ~/.dfs/config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, _ := cmd.Flags().GetString("key")
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			cfg.APIKey = key
			return client.Save(cfgPath, cfg)
		},
	}
	cmd.Flags().String("key", "", "api key")
	return cmd
}
