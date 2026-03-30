package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	AppName             = "Duke Squad"
	BinaryName          = "duke-squad"
	ShortCommandName    = "ds"
	ConfigDirName       = ".duke-squad"
	LegacyConfigDirName = ".claude-squad"
	ReleaseRepository   = "itsLighty/duke-squad"
	LegacyReleaseRepo   = "smtg-ai/claude-squad"
)

var migratableConfigFiles = []string{
	ConfigFileName,
	StateFileName,
	InstancesFileName,
}

func getConfigDirForHome(homeDir string) string {
	return filepath.Join(homeDir, ConfigDirName)
}

func getLegacyConfigDirForHome(homeDir string) string {
	return filepath.Join(homeDir, LegacyConfigDirName)
}

func ensureConfigDirMigrated(homeDir string) error {
	newDir := getConfigDirForHome(homeDir)
	if _, err := os.Stat(newDir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect config directory %s: %w", newDir, err)
	}

	legacyDir := getLegacyConfigDirForHome(homeDir)
	if _, err := os.Stat(legacyDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect legacy config directory %s: %w", legacyDir, err)
	}

	if err := os.MkdirAll(newDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", newDir, err)
	}

	for _, name := range migratableConfigFiles {
		src := filepath.Join(legacyDir, name)
		dst := filepath.Join(newDir, name)
		if err := copyFileIfExists(src, dst); err != nil {
			return err
		}
	}

	return nil
}

func copyFileIfExists(srcPath string, dstPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect %s: %w", srcPath, err)
	}
	if info.IsDir() {
		return nil
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
	}

	return nil
}
