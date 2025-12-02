package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/yanet-platform/monalive/internal/app"
	"github.com/yanet-platform/monalive/internal/monitoring/xlog"
	"github.com/yanet-platform/monalive/internal/utils/exp"
)

func main() {
	// TODO: add some lind of app version.

	// Create a new command with the application name, version, and the exec
	// function.
	var configPath string
	cmd := &cobra.Command{
		Use:   path.Base(os.Args[0]),
		Short: "monalive",
		// TODO:
		// Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			if err := exec(configPath); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		},
	}

	// Add a flag to specify the path to the config file.
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to the config file (required).")
	if err := cmd.MarkFlagRequired("config"); err != nil {
		panic("Logic error: `config` flag not exists in the program")
	}

	// Execute the command. If an error occurs, print it and exit with a
	// non-zero status code.
	if err := cmd.Execute(); err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		os.Exit(1)
	}
}

func exec(configPath string) error {
	// Create a base context.
	ctx := context.Background()

	// Load the application configuration from the specified config path.
	config, err := app.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	exp.ExperimentalFeatures(config.Experiments)

	logger, err := xlog.New(config.Logger)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	// Log the start of the application with the current configuration.
	//
	// TODO: print app version here.
	logger.Info(
		"starting monalive",
		slog.Any("config", config),
	)

	// Create an error group with a derived context for managing goroutines.
	wg, ctx := errgroup.WithContext(ctx)

	// Add a goroutine to the error group that waits for an interruption signal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	wg.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s := <-ch:
			return errors.New(s.String())
		}
	})

	// Initialize the main application logic.
	monalive, err := app.New(config, logger)
	if err != nil {
		return fmt.Errorf("failed to init monalive: %w", err)
	}

	// Add a goroutine to the error group that runs the main application logic.
	wg.Go(func() error {
		return monalive.Run(ctx)
	})

	// Wait for all goroutines in the error group to complete.
	return wg.Wait()
}
