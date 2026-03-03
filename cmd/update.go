package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	githubOwner = "smcronin"
	githubRepo  = "uspto-cli"
)

var (
	updateCheckFlag   bool
	updateForceFlag   bool
	updateVersionFlag string
)

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Name    string               `json:"name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var updateCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"self-update"},
	Short:   "Update uspto-cli from GitHub Releases",
	Long: `Update uspto-cli from GitHub Releases.

By default, this fetches the latest release for your OS/arch, verifies the
checksum, and replaces the current executable.

Use --check to only show current/latest versions without installing.`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateCheckFlag, "check", false, "Check latest version without installing")
	updateCmd.Flags().BoolVar(&updateForceFlag, "force", false, "Install even when already on target version")
	updateCmd.Flags().StringVar(&updateVersionFlag, "version", "", "Target version tag (e.g. v0.1.2); default is latest")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), updateNetworkTimeout())
	defer cancel()

	release, err := fetchGitHubRelease(ctx, updateVersionFlag)
	if err != nil {
		return err
	}

	currentVersion := normalizeVersion(version)
	targetVersion := normalizeVersion(release.TagName)
	upToDate := currentVersion != "" && currentVersion == targetVersion

	assetName := expectedArchiveName(release.TagName, runtime.GOOS, runtime.GOARCH)
	archiveAsset, ok := findReleaseAssetByName(release.Assets, assetName)
	if !ok {
		return fmt.Errorf("release %s does not include asset %q for %s/%s", release.TagName, assetName, runtime.GOOS, runtime.GOARCH)
	}

	checksumAsset, hasChecksums := findReleaseAssetByName(release.Assets, "checksums.txt")
	execPath, _ := currentExecutablePath()

	if updateCheckFlag || (upToDate && !updateForceFlag) {
		result := map[string]interface{}{
			"currentVersion": currentVersionOrUnknown(currentVersion),
			"latestVersion":  targetVersion,
			"upToDate":       upToDate,
			"os":             runtime.GOOS,
			"arch":           runtime.GOARCH,
			"asset":          assetName,
			"executable":     execPath,
		}
		if flagFormat == "json" || flagFormat == "ndjson" || flagFormat == "csv" {
			outputResult(cmd, result, nil)
			return nil
		}
		if upToDate && !updateForceFlag {
			fmt.Fprintf(os.Stdout, "Already up to date: %s\n", targetVersion)
		} else {
			fmt.Fprintf(os.Stdout, "Current: %s\nLatest:  %s\n", currentVersionOrUnknown(currentVersion), targetVersion)
		}
		fmt.Fprintf(os.Stdout, "Asset:   %s (%s/%s)\n", assetName, runtime.GOOS, runtime.GOARCH)
		if execPath != "" {
			fmt.Fprintf(os.Stdout, "Binary:  %s\n", execPath)
		}
		return nil
	}

	if execPath == "" {
		return fmt.Errorf("could not determine executable path")
	}

	tmpDir, err := os.MkdirTemp("", "uspto-cli-update-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	progress(fmt.Sprintf("Downloading %s...", assetName))
	if err := downloadToFile(ctx, archiveAsset.BrowserDownloadURL, archivePath); err != nil {
		return err
	}

	if hasChecksums {
		progress("Verifying checksum...")
		checksumPath := filepath.Join(tmpDir, "checksums.txt")
		if err := downloadToFile(ctx, checksumAsset.BrowserDownloadURL, checksumPath); err != nil {
			return fmt.Errorf("downloading checksums: %w", err)
		}
		if err := verifyFileChecksum(checksumPath, assetName, archivePath); err != nil {
			return err
		}
	}

	progress("Extracting archive...")
	newBinPath, err := extractBinaryFromArchive(archivePath, tmpDir)
	if err != nil {
		return err
	}

	if flagDryRun {
		if flagFormat == "json" || flagFormat == "ndjson" || flagFormat == "csv" {
			outputResult(cmd, map[string]interface{}{
				"currentVersion":  currentVersionOrUnknown(currentVersion),
				"targetVersion":   targetVersion,
				"asset":           assetName,
				"executable":      execPath,
				"downloadedTo":    archivePath,
				"extractedBinary": newBinPath,
				"dryRun":          true,
			}, nil)
			return nil
		}
		fmt.Fprintf(os.Stdout, "Dry run: would replace %s with %s\n", execPath, newBinPath)
		return nil
	}

	scheduled := false
	if runtime.GOOS == "windows" {
		progress("Scheduling Windows binary swap...")
		if err := scheduleWindowsBinarySwap(newBinPath, execPath); err != nil {
			return err
		}
		scheduled = true
	} else {
		progress("Replacing executable...")
		if err := replaceExecutableNow(newBinPath, execPath); err != nil {
			return err
		}
	}

	result := map[string]interface{}{
		"currentVersion": currentVersionOrUnknown(currentVersion),
		"targetVersion":  targetVersion,
		"asset":          assetName,
		"executable":     execPath,
		"updated":        !scheduled,
		"scheduled":      scheduled,
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
	}

	if flagFormat == "json" || flagFormat == "ndjson" || flagFormat == "csv" {
		outputResult(cmd, result, nil)
		return nil
	}

	if scheduled {
		fmt.Fprintf(os.Stdout, "Update scheduled to %s.\n", targetVersion)
		fmt.Fprintln(os.Stdout, "Exit and run `uspto --version` again in a new shell to confirm.")
	} else {
		fmt.Fprintf(os.Stdout, "Updated to %s.\n", targetVersion)
	}

	return nil
}

func updateNetworkTimeout() time.Duration {
	seconds := flagTimeout
	if seconds < 120 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func fetchGitHubRelease(ctx context.Context, requestedTag string) (*githubRelease, error) {
	baseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", githubOwner, githubRepo)
	endpoint := baseURL + "/latest"
	if strings.TrimSpace(requestedTag) != "" {
		tag := strings.TrimSpace(requestedTag)
		if !strings.HasPrefix(strings.ToLower(tag), "v") {
			tag = "v" + tag
		}
		endpoint = baseURL + "/tags/" + url.PathEscape(tag)
	}

	body, err := httpGet(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("decoding GitHub release response: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("unexpected GitHub release response: missing tag_name")
	}
	return &rel, nil
}

func httpGet(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "uspto-cli-update")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, msg)
	}

	return body, nil
}

func expectedArchiveName(tag, goos, goarch string) string {
	ver := normalizeVersion(tag)
	if goos == "windows" {
		return fmt.Sprintf("uspto-cli_%s_%s_%s.zip", ver, goos, goarch)
	}
	return fmt.Sprintf("uspto-cli_%s_%s_%s.tar.gz", ver, goos, goarch)
}

func findReleaseAssetByName(assets []githubReleaseAsset, name string) (githubReleaseAsset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return githubReleaseAsset{}, false
}

func downloadToFile(ctx context.Context, rawURL, outPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("creating download request: %w", err)
	}
	req.Header.Set("User-Agent", "uspto-cli-update")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("download failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing download: %w", err)
	}
	return nil
}

func verifyFileChecksum(checksumsPath, assetName, assetPath string) error {
	checksumData, err := os.ReadFile(checksumsPath)
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	expected, ok := lookupChecksum(string(checksumData), assetName)
	if !ok {
		return fmt.Errorf("checksums file does not contain %s", assetName)
	}

	got, err := fileSHA256(assetPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, got) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, expected, got)
	}
	return nil
}

func lookupChecksum(checksums, filename string) (string, bool) {
	lines := strings.Split(checksums, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		name := strings.TrimPrefix(fields[1], "*")
		if name == filename {
			return hash, true
		}
	}
	return "", false
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file for hash: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractBinaryFromArchive(archivePath, outDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractBinaryFromZip(archivePath, outDir)
	}
	return extractBinaryFromTarGz(archivePath, outDir)
}

func extractBinaryFromZip(archivePath, outDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening zip archive: %w", err)
	}
	defer r.Close()

	binName := binaryNameForRuntime()
	for _, f := range r.File {
		if filepath.Base(f.Name) != binName {
			continue
		}
		in, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening zip entry: %w", err)
		}
		defer in.Close()

		outPath := filepath.Join(outDir, "uspto-cli.update.bin")
		out, err := os.Create(outPath)
		if err != nil {
			return "", fmt.Errorf("creating extracted binary: %w", err)
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return "", fmt.Errorf("extracting zip binary: %w", err)
		}
		out.Close()
		if err := os.Chmod(outPath, 0755); err != nil {
			return "", fmt.Errorf("setting executable mode: %w", err)
		}
		return outPath, nil
	}

	return "", fmt.Errorf("binary %q not found in %s", binName, archivePath)
}

func extractBinaryFromTarGz(archivePath, outDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("opening gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	binName := binaryNameForRuntime()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading tar archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != binName {
			continue
		}

		outPath := filepath.Join(outDir, "uspto-cli.update.bin")
		out, err := os.Create(outPath)
		if err != nil {
			return "", fmt.Errorf("creating extracted binary: %w", err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", fmt.Errorf("extracting tar binary: %w", err)
		}
		out.Close()
		if err := os.Chmod(outPath, 0755); err != nil {
			return "", fmt.Errorf("setting executable mode: %w", err)
		}
		return outPath, nil
	}

	return "", fmt.Errorf("binary %q not found in %s", binName, archivePath)
}

func binaryNameForRuntime() string {
	if runtime.GOOS == "windows" {
		return "uspto-cli.exe"
	}
	return "uspto-cli"
}

func replaceExecutableNow(newBinPath, execPath string) error {
	dir := filepath.Dir(execPath)
	tmpDest := filepath.Join(dir, filepath.Base(execPath)+".new")
	if err := copyFile(newBinPath, tmpDest, 0755); err != nil {
		return err
	}
	if err := os.Rename(tmpDest, execPath); err != nil {
		return fmt.Errorf("replacing executable: %w", err)
	}
	return nil
}

func scheduleWindowsBinarySwap(newBinPath, execPath string) error {
	targetNew := execPath + ".new"
	if err := copyFile(newBinPath, targetNew, 0755); err != nil {
		return err
	}

	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("uspto-cli-update-%d.ps1", time.Now().UnixNano()))
	script := windowsSwapScript(scriptPath, targetNew, execPath, os.Getpid())
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		return fmt.Errorf("writing update script: %w", err)
	}

	psExe, err := exec.LookPath("powershell")
	if err != nil {
		return fmt.Errorf("powershell not found; cannot self-update on Windows automatically")
	}
	cmd := exec.Command(psExe, "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching update script: %w", err)
	}
	return nil
}

func windowsSwapScript(scriptPath, src, dst string, pid int) string {
	quote := func(s string) string {
		return strings.ReplaceAll(s, `'`, `''`)
	}
	return fmt.Sprintf(
		"$pidToWait = %d\n$src = '%s'\n$dst = '%s'\n$self = '%s'\nfor ($i = 0; $i -lt 300; $i++) {\n  if (-not (Get-Process -Id $pidToWait -ErrorAction SilentlyContinue)) { break }\n  Start-Sleep -Milliseconds 200\n}\nfor ($i = 0; $i -lt 50; $i++) {\n  try {\n    Copy-Item -Path $src -Destination $dst -Force\n    break\n  } catch {\n    Start-Sleep -Milliseconds 200\n  }\n}\nRemove-Item -Path $src -Force -ErrorAction SilentlyContinue\nRemove-Item -Path $self -Force -ErrorAction SilentlyContinue\n",
		pid, quote(src), quote(dst), quote(scriptPath),
	)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copying file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing destination file: %w", err)
	}
	if err := os.Chmod(dst, mode); err != nil {
		return fmt.Errorf("setting file mode: %w", err)
	}
	return nil
}

func currentExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil && resolved != "" {
		return resolved, nil
	}
	return path, nil
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	return v
}

func currentVersionOrUnknown(v string) string {
	if v == "" || v == "dev" {
		return "dev"
	}
	return v
}
