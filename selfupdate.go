package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const releaseRepo = "Militskiy/chatgpt-update"
const releaseAPI = "https://api.github.com/repos/" + releaseRepo + "/releases/latest"
const exeAsset = "chatgpt-update.exe"
const portableAsset = "chatgpt-update-windows-x64.zip"
const maxArchiveSize = 100 << 20

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	State  string `json:"state"`
	Size   int64  `json:"size"`
}
type releaseInfo struct {
	Tag        string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

var stableTag = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func parseVersion(s string) ([3]uint64, error) {
	var v [3]uint64
	m := stableTag.FindStringSubmatch(s)
	if m == nil {
		return v, fmt.Errorf("unsupported stable version %q", s)
	}
	for i := 0; i < 3; i++ {
		n, e := strconv.ParseUint(m[i+1], 10, 32)
		if e != nil {
			return v, e
		}
		v[i] = n
	}
	return v, nil
}
func newerVersion(a, b string) (bool, error) {
	av, e := parseVersion(a)
	if e != nil {
		return false, e
	}
	bv, e := parseVersion(b)
	if e != nil {
		return false, e
	}
	for i := 0; i < 3; i++ {
		if av[i] != bv[i] {
			return av[i] > bv[i], nil
		}
	}
	return false, nil
}
func validateAssetURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil {
		return e
	}
	if u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.Fragment != "" || !strings.HasPrefix(u.Path, "/"+releaseRepo+"/releases/download/") {
		return errors.New("updater asset is outside the configured HTTPS GitHub Releases repository")
	}
	return nil
}
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return errors.New("too many download redirects")
		}
		host := req.URL.Host
		allowed := host == "github.com" || host == "api.github.com" || host == "release-assets.githubusercontent.com" || host == "objects.githubusercontent.com"
		if req.URL.Scheme != "https" || req.URL.User != nil || !allowed {
			return errors.New("unexpected download redirect")
		}
		return nil
	}}
}
func httpGet(client *http.Client, raw string) (*http.Response, error) {
	req, e := http.NewRequest(http.MethodGet, raw, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "Militskiy-ChatGPT-Update/"+version)
	req.Header.Set("Cache-Control", "no-cache")
	resp, e := client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("network request failed (HTTPS_PROXY is supported): %w", e)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == 404 {
			return nil, errors.New("no published portable release is available yet, or the release was removed")
		}
		return nil, fmt.Errorf("GitHub returned HTTP %d (rate limiting or corporate filtering may apply)", resp.StatusCode)
	}
	return resp, nil
}
func choosePortable(r releaseInfo) (releaseAsset, *releaseAsset, error) {
	if r.Draft || r.Prerelease {
		return releaseAsset{}, nil, errors.New("refusing draft/prerelease updater")
	}
	if _, e := parseVersion(r.Tag); e != nil {
		return releaseAsset{}, nil, e
	}
	var exe releaseAsset
	var checksum *releaseAsset
	for _, a := range r.Assets {
		if a.State != "uploaded" {
			continue
		}
		if a.Name == portableAsset {
			if exe.Name != "" {
				return exe, nil, errors.New("ambiguous portable ZIP assets")
			}
			exe = a
		}
		if a.Name == "SHA256SUMS.txt" {
			if checksum != nil {
				return exe, nil, errors.New("ambiguous checksum assets")
			}
			copy := a
			checksum = &copy
		}
	}
	if exe.Name == "" || exe.Size < 1024 || exe.Size > maxArchiveSize {
		return exe, nil, errors.New("release has no complete Windows x64 portable ZIP (v0.1.0 single-EXE updates are not supported)")
	}
	if e := validateAssetURL(exe.URL); e != nil {
		return exe, nil, e
	}
	if checksum != nil {
		if e := validateAssetURL(checksum.URL); e != nil {
			return exe, nil, e
		}
	}
	return exe, checksum, nil
}
func checksumFromText(text, name string) (string, error) {
	found := ""
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		b, e := hex.DecodeString(fields[0])
		if e != nil || len(b) != 32 || found != "" {
			return "", errors.New("invalid/ambiguous checksum entry")
		}
		found = strings.ToLower(fields[0])
	}
	if found == "" {
		return "", errors.New("checksum entry for chatgpt-update-windows-x64.zip is missing")
	}
	return found, nil
}
func expectedChecksum(client *http.Client, exe releaseAsset, sum *releaseAsset) (string, error) {
	expected := ""
	if strings.HasPrefix(exe.Digest, "sha256:") {
		expected = strings.TrimPrefix(exe.Digest, "sha256:")
		b, e := hex.DecodeString(expected)
		if e != nil || len(b) != 32 {
			return "", errors.New("invalid GitHub asset digest")
		}
		expected = strings.ToLower(expected)
	}
	if sum != nil {
		if sum.Size < 1 || sum.Size > 1<<20 {
			return "", errors.New("unexpected checksum file size")
		}
		resp, e := httpGet(client, sum.URL)
		if e != nil {
			return "", e
		}
		defer resp.Body.Close()
		b, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if e != nil {
			return "", e
		}
		hash, e := checksumFromText(string(b), portableAsset)
		if e != nil {
			return "", e
		}
		if expected != "" && expected != hash {
			return "", errors.New("GitHub digest and release checksum disagree")
		}
		expected = hash
	}
	if expected == "" {
		return "", errors.New("no SHA-256 digest/checksum published; refusing self-update")
	}
	return expected, nil
}

type downloadProgress struct {
	downloaded, total int64
	last              time.Time
}

func (p *downloadProgress) Write(b []byte) (int, error) {
	p.downloaded += int64(len(b))
	if time.Since(p.last) > 250*time.Millisecond {
		frac := float64(p.downloaded) / float64(p.total)
		filled := int(frac * 28)
		if filled > 28 {
			filled = 28
		}
		fmt.Printf("\r[>>] Download updater [%s%s] %5.1f%% | %.1f / %.1f MiB", strings.Repeat("#", filled), strings.Repeat("-", 28-filled), frac*100, float64(p.downloaded)/(1<<20), float64(p.total)/(1<<20))
		p.last = time.Now()
	}
	return len(b), nil
}
func downloadArchive(client *http.Client, asset releaseAsset, expected, destination string) error {
	resp, e := httpGet(client, asset.URL)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	f, e := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	h := sha256.New()
	progress := &downloadProgress{total: asset.Size}
	n, copyErr := io.Copy(io.MultiWriter(f, h, progress), io.LimitReader(resp.Body, asset.Size+1))
	closeErr := f.Close()
	fmt.Println()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n != asset.Size {
		return errors.New("download size does not match release metadata")
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return errors.New("SHA-256 verification failed; existing EXE not changed")
	}

	return nil
}

type replacementPlan struct {
	ParentPID  int          `json:"parentPid"`
	Target     string       `json:"target"`
	Candidate  string       `json:"candidate"`
	NewVersion string       `json:"newVersion"`
	Previous   string       `json:"previous"`
	Log        string       `json:"log"`
	LockName   string       `json:"lockName"`
	Files      []fileRecord `json:"files"`
}

func selfUpdate(yes bool) error {
	fmt.Println("[>>] Checking Militskiy/chatgpt-update public stable releases...")
	client := newHTTPClient()
	resp, e := httpGet(client, releaseAPI)
	if e != nil {
		return e
	}
	var release releaseInfo
	e = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release)
	resp.Body.Close()
	if e != nil {
		return e
	}
	if release.Draft || release.Prerelease {
		return errors.New("release is not stable")
	}
	newer, e := newerVersion(release.Tag, version)
	if e != nil {
		return e
	}
	fmt.Println("Installed updater:", version)
	fmt.Println("Released updater: ", release.Tag)
	if !newer {
		fmt.Println("[OK] No newer stable updater release.")
		return nil
	}
	asset, sum, e := choosePortable(release)
	if e != nil {
		return e
	}
	target, e := os.Executable()
	if e != nil {
		return e
	}
	root := filepath.Dir(target)
	if isReparsePoint(target) {
		return errors.New("self-update does not replace links")
	}
	if e = verifyScripts(root); e != nil {
		return e
	}
	fmt.Println("Portable folder to update:", root)
	fmt.Println("The complete ZIP replaces this EXE and its companion scripts. ChatGPT/data are unchanged.")
	fmt.Println("The package is unsigned. HTTPS/checksums rely on the security of this repository.")
	if !yes && !confirm("Update the updater?") {
		return nil
	}
	expected, e := expectedChecksum(client, asset, sum)
	if e != nil {
		return e
	}
	work, e := os.MkdirTemp(root, ".chatgpt-update-stage-")
	if e != nil {
		return fmt.Errorf("portable folder must be writable: %w", e)
	}
	handedOff := false
	defer func() {
		if !handedOff {
			os.RemoveAll(work)
		}
	}()
	archive := filepath.Join(work, portableAsset)
	if e = downloadArchive(client, asset, expected, archive); e != nil {
		return e
	}
	candidate := filepath.Join(work, "package")
	next := strings.TrimPrefix(release.Tag, "v")
	records, e := extractPortable(archive, candidate, next)
	if e != nil {
		return e
	}
	fmt.Println("[OK] ZIP hash, file set, component hashes, version and x64 EXE format verified.")
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return errors.New("LOCALAPPDATA is not set")
	}
	logs := filepath.Join(local, "ChatGPTUpdater", "Logs")
	if e = os.MkdirAll(logs, 0700); e != nil {
		return e
	}
	lockName, e := operationLockName()
	if e != nil {
		return e
	}
	id := uniqueName()
	plan := replacementPlan{os.Getpid(), target, candidate, next,
		filepath.Join(root, ".chatgpt-update-previous-"+id),
		filepath.Join(logs, "self-update-"+id+".log"), lockName, records}
	b, e := json.MarshalIndent(plan, "", "  ")
	if e != nil {
		return e
	}
	planPath := filepath.Join(work, "replacement.json")
	if e = os.WriteFile(planPath, b, 0600); e != nil {
		return e
	}
	if e = startReplacementHelper(planPath); e != nil {
		return e
	}
	handedOff = true
	fmt.Println("[>>] A visible helper window will finish after this updater exits.")
	fmt.Println("If Windows blocks the helper, nothing is overwritten. Ask IT to approve the scripts.")
	fmt.Println("Log:", plan.Log)
	fmt.Println("Run chatgpt-update again after completion. Previous package files are retained for rollback.")
	return errUpdating
}
