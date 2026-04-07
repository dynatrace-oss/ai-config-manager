package test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScript_UsesReleaseTagForDownloadsAndPlainVersionForAssets(t *testing.T) {
	release := createFakeUnixInstallerRelease(t, "v1.2.3")
	installDir := t.TempDir()

	output := runShellInstallerScript(t, release, installDir, map[string]string{})

	assertFileExists(t, filepath.Join(installDir, "aimgr"))
	assertOutputContains(t, output, "Installed aimgr")

	urls := readLoggedURLs(t, release.logPath)
	if len(urls) != 3 {
		t.Fatalf("expected 3 download URLs, got %d: %v", len(urls), urls)
	}

	assertContainsURL(t, urls, release.apiURL)
	assertContainsURL(t, urls, release.assetURL)
	assertContainsURL(t, urls, release.checksumsURL)

	for _, url := range urls {
		if strings.Contains(url, "aimgr_v1.2.3_") {
			t.Fatalf("installer requested asset with tagged version in filename: %s", url)
		}
	}
}

func TestInstallScript_ExplicitVersionAcceptsTaggedOrPlain(t *testing.T) {
	for _, requestedVersion := range []string{"1.2.3", "v1.2.3"} {
		t.Run(requestedVersion, func(t *testing.T) {
			release := createFakeUnixInstallerRelease(t, "v1.2.3")
			installDir := t.TempDir()

			output := runShellInstallerScript(t, release, installDir, map[string]string{
				"AIMGR_VERSION": requestedVersion,
			})

			assertFileExists(t, filepath.Join(installDir, "aimgr"))
			assertOutputContains(t, output, "Downloading aimgr 1.2.3")

			urls := readLoggedURLs(t, release.logPath)
			if len(urls) != 2 {
				t.Fatalf("expected 2 download URLs for explicit version, got %d: %v", len(urls), urls)
			}

			assertContainsURL(t, urls, release.assetURL)
			assertContainsURL(t, urls, release.checksumsURL)

			for _, url := range urls {
				if url == release.apiURL {
					t.Fatalf("installer should not query latest-release API when AIMGR_VERSION is set: %v", urls)
				}
			}
		})
	}
}

type fakeUnixInstallerRelease struct {
	repo          string
	tag           string
	plainVersion  string
	apiURL        string
	assetURL      string
	checksumsURL  string
	assetPath     string
	checksumsPath string
	metadataPath  string
	logPath       string
	binDir        string
}

func createFakeUnixInstallerRelease(t *testing.T, tag string) fakeUnixInstallerRelease {
	t.Helper()

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}

	plainVersion := strings.TrimPrefix(strings.TrimPrefix(tag, "v"), "V")
	repo := "example/test-repo"
	releaseBaseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, tag)
	assetName := fmt.Sprintf("aimgr_%s_linux_amd64.tar.gz", plainVersion)
	assetPath := filepath.Join(root, assetName)
	createTarGz(t, assetPath, "aimgr", []byte("#!/bin/sh\nexit 0\n"), 0755)

	checksum := sha256File(t, assetPath)
	checksumsPath := filepath.Join(root, "checksums.txt")
	if err := os.WriteFile(checksumsPath, []byte(fmt.Sprintf("%s  %s\n", checksum, assetName)), 0644); err != nil {
		t.Fatalf("write checksums.txt: %v", err)
	}

	metadataPath := filepath.Join(root, "latest-release.json")
	if err := os.WriteFile(metadataPath, []byte(fmt.Sprintf(`{"tag_name":%q}`+"\n", tag)), 0644); err != nil {
		t.Fatalf("write latest release metadata: %v", err)
	}

	logPath := filepath.Join(root, "curl.log")
	writeFakeCurl(t, filepath.Join(binDir, "curl"))

	return fakeUnixInstallerRelease{
		repo:          repo,
		tag:           tag,
		plainVersion:  plainVersion,
		apiURL:        fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo),
		assetURL:      releaseBaseURL + "/" + assetName,
		checksumsURL:  releaseBaseURL + "/checksums.txt",
		assetPath:     assetPath,
		checksumsPath: checksumsPath,
		metadataPath:  metadataPath,
		logPath:       logPath,
		binDir:        binDir,
	}
}

func runShellInstallerScript(t *testing.T, release fakeUnixInstallerRelease, installDir string, extraEnv map[string]string) string {
	t.Helper()

	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}

	scriptPath := filepath.Join(projectRoot, "scripts", "install.sh")
	cmd := exec.Command("sh", scriptPath)
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"AIMGR_GITHUB_REPO="+release.repo,
		"AIMGR_INSTALL_DIR="+installDir,
		"PATH="+release.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_API_URL="+release.apiURL,
		"FAKE_RELEASE_JSON="+release.metadataPath,
		"FAKE_RELEASE_BASE_URL="+strings.TrimSuffix(release.assetURL, "/"+filepath.Base(release.assetPath)),
		"FAKE_ASSET_NAME="+filepath.Base(release.assetPath),
		"FAKE_ASSET_FILE="+release.assetPath,
		"FAKE_CHECKSUMS_FILE="+release.checksumsPath,
		"FAKE_CURL_LOG="+release.logPath,
	)
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\nOutput:\n%s", err, output)
	}

	return string(output)
}

func writeFakeCurl(t *testing.T, path string) {
	t.Helper()

	content := `#!/bin/sh
set -eu

url=""
dest=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        -o)
            dest=$2
            shift 2
            ;;
        -*)
            shift
            ;;
        *)
            url=$1
            shift
            ;;
    esac
done

printf '%s\n' "$url" >> "$FAKE_CURL_LOG"

case "$url" in
    "$FAKE_API_URL")
        cp "$FAKE_RELEASE_JSON" "$dest"
        ;;
    "$FAKE_RELEASE_BASE_URL/$FAKE_ASSET_NAME")
        cp "$FAKE_ASSET_FILE" "$dest"
        ;;
    "$FAKE_RELEASE_BASE_URL/checksums.txt")
        cp "$FAKE_CHECKSUMS_FILE" "$dest"
        ;;
    *)
        printf 'unexpected URL: %s\n' "$url" >&2
        exit 22
        ;;
esac
`

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write fake curl: %v", err)
	}
}

func createTarGz(t *testing.T, archivePath, fileName string, content []byte, mode int64) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	gz := gzip.NewWriter(file)

	tw := tar.NewWriter(gz)

	header := &tar.Header{
		Name: fileName,
		Mode: mode,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}
}

func sha256File(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file for checksum: %v", err)
	}
	checksum := sha256.Sum256(content)
	return fmt.Sprintf("%x", checksum)
}

func readLoggedURLs(t *testing.T, path string) []string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read logged URLs: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func assertContainsURL(t *testing.T, urls []string, want string) {
	t.Helper()

	for _, url := range urls {
		if url == want {
			return
		}
	}

	t.Fatalf("expected URL %q in %v", want, urls)
}
