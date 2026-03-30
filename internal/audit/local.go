package audit

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func DiscoverLocalRepositories(path string, maxDepth int) ([]Repository, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	if isGitRepository(root) {
		return []Repository{inferLocalRepository(root)}, nil
	}

	if maxDepth < 1 {
		maxDepth = 1
	}

	var repositories []Repository
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}

		if current != root {
			rel, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			depth := strings.Count(rel, string(os.PathSeparator)) + 1
			if depth > maxDepth {
				return filepath.SkipDir
			}
		}

		if entry.Name() == ".git" {
			return filepath.SkipDir
		}

		if isGitRepository(current) {
			repositories = append(repositories, inferLocalRepository(current))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover repositories: %w", err)
	}

	return repositories, nil
}

func LoadLocalWorkflowFiles(repo Repository) ([]WorkflowFile, error) {
	workflowsDir := filepath.Join(repo.Path, ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workflows directory: %w", err)
	}

	files := make([]WorkflowFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		workflowPath := filepath.Join(".github", "workflows", name)
		fullPath := filepath.Join(repo.Path, workflowPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read workflow %s: %w", fullPath, err)
		}

		files = append(files, WorkflowFile{
			Path:    filepath.ToSlash(workflowPath),
			Content: content,
		})
	}

	return files, nil
}

func inferLocalRepository(root string) Repository {
	repo := Repository{
		Name:   filepath.Base(root),
		Path:   root,
		Source: string(ModeLocal),
	}

	configPath := gitConfigPath(root)
	if configPath == "" {
		return repo
	}

	fullName := parseOriginFullName(configPath)
	if fullName == "" {
		return repo
	}

	repo.FullName = fullName
	parts := strings.Split(fullName, "/")
	if len(parts) == 2 && parts[1] != "" {
		repo.Name = parts[1]
	}

	return repo
}

func isGitRepository(root string) bool {
	gitPath := filepath.Join(root, ".git")
	if info, err := os.Stat(gitPath); err == nil {
		return info.IsDir() || !info.IsDir()
	}
	return false
}

func gitConfigPath(root string) string {
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}

	if info.IsDir() {
		configPath := filepath.Join(gitPath, "config")
		if fileExists(configPath) {
			return configPath
		}
		return ""
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir:") {
		return ""
	}

	gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}

	configPath := filepath.Join(gitDir, "config")
	if fileExists(configPath) {
		return configPath
	}

	commonDirPath := filepath.Join(gitDir, "commondir")
	commonDirBytes, err := os.ReadFile(commonDirPath)
	if err != nil {
		return ""
	}

	commonDir := strings.TrimSpace(string(commonDirBytes))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}
	configPath = filepath.Join(commonDir, "config")
	if fileExists(configPath) {
		return configPath
	}

	return ""
}

func parseOriginFullName(configPath string) string {
	file, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inOrigin := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "["):
			inOrigin = line == `[remote "origin"]`
		case inOrigin && strings.HasPrefix(line, "url ="):
			url := strings.TrimSpace(strings.TrimPrefix(line, "url ="))
			return parseRepositoryFullName(url)
		}
	}

	return ""
}

func parseRepositoryFullName(remoteURL string) string {
	url := strings.TrimSpace(strings.TrimSuffix(remoteURL, ".git"))
	if url == "" {
		return ""
	}

	switch {
	case strings.Contains(url, "://"):
		parts := strings.Split(url, "://")
		if len(parts) != 2 {
			return ""
		}
		pathPart := parts[1]
		slash := strings.Index(pathPart, "/")
		if slash == -1 {
			return ""
		}
		return strings.Trim(pathPart[slash+1:], "/")
	case strings.Contains(url, ":"):
		parts := strings.SplitN(url, ":", 2)
		return strings.Trim(parts[1], "/")
	default:
		return ""
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
