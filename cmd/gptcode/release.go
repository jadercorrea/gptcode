package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var releaseCmd = &cobra.Command{
	Use:   "release [major|minor|patch]",
	Short: "Create a new release (bumps version, creates tag, pushes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bumpType := args[0]
		if bumpType != "major" && bumpType != "minor" && bumpType != "patch" {
			return fmt.Errorf("invalid bump type: %s (use major, minor, or patch)", bumpType)
		}

		fmt.Printf("Creating %s release...\n", bumpType)

		// Get current version
		currentVersion, err := getCurrentVersion()
		if err != nil {
			return fmt.Errorf("failed to get current version: %w", err)
		}
		fmt.Printf("Current version: %s\n", currentVersion)

		// Bump version
		newVersion, err := bumpVersion(currentVersion, bumpType)
		if err != nil {
			return fmt.Errorf("failed to bump version: %w", err)
		}
		fmt.Printf("New version: %s\n", newVersion)

		// Update version in code
		if err := updateVersion(newVersion); err != nil {
			return fmt.Errorf("failed to update version: %w", err)
		}

		// Generate changelog
		changelog, err := generateChangelog(currentVersion)
		if err != nil {
			fmt.Printf("Warning: failed to generate changelog: %v\n", err)
			changelog = "Release " + newVersion
		}

		// Create git tag
		if err := createTag(newVersion, changelog); err != nil {
			return fmt.Errorf("failed to create tag: %w", err)
		}

		// Push to trigger release
		fmt.Println("\nPushing to trigger release...")
		if err := push(); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}

		fmt.Printf(`
✅ Release %s created and pushed!

The release will be built and published automatically.
Check https://github.com/jadercorrea/gptcode/releases
`, newVersion)

		return nil
	},
}

func getCurrentVersion() (string, error) {
	// Try to get from git tags first
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
	if err == nil {
		version := strings.TrimSpace(string(out))
		if strings.HasPrefix(version, "v") {
			return version, nil
		}
		return "v" + version, nil
	}

	// Fallback to reading version file or using default
	return "v0.0.0", nil
}

func bumpVersion(current, bumpType string) (string, error) {
	// Remove v prefix
	version := strings.TrimPrefix(current, "v")

	// Parse version numbers
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid version format: %s", current)
	}

	var major, minor, patch int
	fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)

	switch bumpType {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func updateVersion(version string) error {
	// Update version in cmd/gptcode/main.go
	versionRE := regexp.MustCompile(`var version = "v[^"]+"`)

	data, err := os.ReadFile("cmd/gptcode/main.go")
	if err != nil {
		return err
	}

	newData := versionRE.ReplaceAllString(string(data), fmt.Sprintf(`var version = "%s"`, version))

	if err := os.WriteFile("cmd/gptcode/main.go", []byte(newData), 0644); err != nil {
		return err
	}

	// Also update goreleaser.yaml if exists
	goreleaserFiles := []string{
		"goreleaser.yaml",
		".goreleaser.yaml",
		"release.md",
	}

	for _, f := range goreleaserFiles {
		if data, err := os.ReadFile(f); err == nil {
			newData := versionRE.ReplaceAllString(string(data), fmt.Sprintf(`var version = "%s"`, version))
			os.WriteFile(f, []byte(newData), 0644)
		}
	}

	return nil
}

func generateChangelog(sinceVersion string) (string, error) {
	out, err := exec.Command("git", "log", sinceVersion+"..HEAD", "--pretty=format:- %s (%h)", "--no-merges").Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var changes []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			changes = append(changes, line)
		}
	}

	if len(changes) == 0 {
		return "No changes since " + sinceVersion, nil
	}

	return strings.Join(changes, "\n"), nil
}

func createTag(version, changelog string) error {
	// Create annotated tag
	cmd := exec.Command("git", "tag", "-a", version, "-m", changelog)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	fmt.Printf("Created tag: %s\n", version)
	return nil
}

func push() error {
	cmd := exec.Command("git", "push", "origin", "main", "--tags")
	return cmd.Run()
}
