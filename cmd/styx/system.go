package main

import (
	"context"
	"fmt"

	"github.com/kessler-frost/styx/driver/container"
	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Container system management commands",
	Long:  `Commands for managing the container runtime system, including disk usage and cleanup.`,
}

var systemDfCmd = &cobra.Command{
	Use:   "df",
	Short: "Show container disk usage",
	Long:  `Display disk usage for images, containers, and volumes.`,
	RunE:  runSystemDf,
}

var systemPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove unused images",
	Long:  `Remove unused container images to free disk space.`,
	RunE:  runSystemPrune,
}

var systemResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Stop all containers, remove volumes, and prune images",
	Long:  `Full cleanup: stops all containers, removes all volumes, and prunes all images.`,
	RunE:  runSystemReset,
}

func init() {
	rootCmd.AddCommand(systemCmd)
	systemCmd.AddCommand(systemDfCmd)
	systemCmd.AddCommand(systemPruneCmd)
	systemCmd.AddCommand(systemResetCmd)
}

func humanizeBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func runSystemDf(cmd *cobra.Command, args []string) error {
	client := container.NewClient("/usr/local/bin/container")
	ctx := context.Background()

	usage, err := client.DiskUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get disk usage: %w", err)
	}

	fmt.Println("Container Disk Usage")
	fmt.Println("--------------------")
	fmt.Println()

	// Images
	fmt.Printf("Images:\n")
	fmt.Printf("  Total:       %d\n", usage.Images.Total)
	fmt.Printf("  Active:      %d\n", usage.Images.Active)
	fmt.Printf("  Size:        %s\n", humanizeBytes(usage.Images.SizeInBytes))
	fmt.Printf("  Reclaimable: %s\n", humanizeBytes(usage.Images.Reclaimable))
	fmt.Println()

	// Containers
	fmt.Printf("Containers:\n")
	fmt.Printf("  Total:       %d\n", usage.Containers.Total)
	fmt.Printf("  Active:      %d\n", usage.Containers.Active)
	fmt.Printf("  Size:        %s\n", humanizeBytes(usage.Containers.SizeInBytes))
	fmt.Printf("  Reclaimable: %s\n", humanizeBytes(usage.Containers.Reclaimable))
	fmt.Println()

	// Volumes
	fmt.Printf("Volumes:\n")
	fmt.Printf("  Total:       %d\n", usage.Volumes.Total)
	fmt.Printf("  Active:      %d\n", usage.Volumes.Active)
	fmt.Printf("  Size:        %s\n", humanizeBytes(usage.Volumes.SizeInBytes))
	fmt.Printf("  Reclaimable: %s\n", humanizeBytes(usage.Volumes.Reclaimable))
	fmt.Println()

	// Total
	totalSize := usage.Images.SizeInBytes + usage.Containers.SizeInBytes + usage.Volumes.SizeInBytes
	totalReclaimable := usage.Images.Reclaimable + usage.Containers.Reclaimable + usage.Volumes.Reclaimable
	fmt.Printf("Total Size:        %s\n", humanizeBytes(totalSize))
	fmt.Printf("Total Reclaimable: %s\n", humanizeBytes(totalReclaimable))

	return nil
}

func runSystemPrune(cmd *cobra.Command, args []string) error {
	client := container.NewClient("/usr/local/bin/container")
	ctx := context.Background()

	// Get usage before prune
	beforeUsage, err := client.DiskUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get disk usage: %w", err)
	}

	fmt.Println("Pruning unused images...")
	if err := client.Prune(ctx); err != nil {
		return fmt.Errorf("failed to prune images: %w", err)
	}

	// Get usage after prune
	afterUsage, err := client.DiskUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get disk usage after prune: %w", err)
	}

	freed := beforeUsage.Images.SizeInBytes - afterUsage.Images.SizeInBytes
	fmt.Printf("Freed %s of disk space\n", humanizeBytes(freed))

	return nil
}

func runSystemReset(cmd *cobra.Command, args []string) error {
	client := container.NewClient("/usr/local/bin/container")
	ctx := context.Background()

	// Get initial disk usage
	beforeUsage, err := client.DiskUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get disk usage: %w", err)
	}

	// Stop and remove all containers
	fmt.Println("Stopping and removing all containers...")
	containers, err := client.List(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		_ = client.Stop(ctx, c.Configuration.ID)
		_ = client.Remove(ctx, c.Configuration.ID)
	}
	fmt.Printf("  Removed %d containers\n", len(containers))

	// Remove all volumes
	fmt.Println("Removing all volumes...")
	volumes, err := client.VolumeList(ctx)
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	for _, v := range volumes {
		_ = client.VolumeRemove(ctx, v.Name)
	}
	fmt.Printf("  Removed %d volumes\n", len(volumes))

	// Prune all images
	fmt.Println("Pruning all images...")
	if err := client.Prune(ctx); err != nil {
		return fmt.Errorf("failed to prune images: %w", err)
	}

	// Get final disk usage
	afterUsage, err := client.DiskUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to get disk usage: %w", err)
	}

	totalFreed := (beforeUsage.Images.SizeInBytes + beforeUsage.Containers.SizeInBytes + beforeUsage.Volumes.SizeInBytes) -
		(afterUsage.Images.SizeInBytes + afterUsage.Containers.SizeInBytes + afterUsage.Volumes.SizeInBytes)

	fmt.Printf("\nFreed %s of disk space\n", humanizeBytes(totalFreed))

	return nil
}
