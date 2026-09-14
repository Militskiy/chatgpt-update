package main

import (
	"crypto/sha256"
	"debug/pe"
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
const maxExeSize = 100 << 20

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
func chooseExecutable(r releaseInfo) (releaseAsset, *releaseAsset, error) {
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
		if a.Name == exeAsset {
			if exe.Name != "" {
				return exe, nil, errors.New("ambiguous EXE assets")
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
	if exe.Name == "" || exe.Size < 1024 || exe.Size > maxExeSize {
		return exe, nil, errors.New("release has no valid Windows x64 portable executable")
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
		return "", errors.New("checksum entry for chatgpt-update.exe is missing")
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
		hash, e := checksumFromText(string(b), exeAsset)
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
func downloadExe(client *http.Client, asset releaseAsset, expected, destination string) error {
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
	binary, e := pe.Open(destination)
	if e != nil {
		return fmt.Errorf("download is not a Windows PE executable: %w", e)
	}
	defer binary.Close()
	if binary.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return errors.New("download is not a Windows x64 executable")
	}
	return nil
}

type replacementPlan struct {
	ParentPID  int    `json:"parentPid"`
	Target     string `json:"target"`
	Candidate  string `json:"candidate"`
	SHA256     string `json:"sha256"`
	NewVersion string `json:"newVersion"`
	Previous   string `json:"previous"`
	Log        string `json:"log"`
}

func selfUpdate(yes bool) error {
	fmt.Println("[>>] Checking Militskiy/chatgpt-update public releases...")
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
	exe, sum, e := chooseExecutable(release)
	if e != nil {
		return e
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
	target, e := os.Executable()
	if e != nil {
		return e
	}
	if isReparsePoint(target) {
		return errors.New("move the real EXE into a writable folder; self-update does not replace links")
	}
	fmt.Println("Executable to replace:", target)
	fmt.Println("The verified release replaces this EXE. ChatGPT and backups will NOT be changed.")
	fmt.Println("Releases are unsigned; SHA-256 checks rely on the security of this GitHub repository.")
	if !yes && !confirm("Update the updater?") {
		return nil
	}
	probe, e := os.CreateTemp(filepath.Dir(target), ".chatgpt-update-write-test-")
	if e != nil {
		return fmt.Errorf("portable folder must be writable: %w", e)
	}
	probe.Close()
	os.Remove(probe.Name())
	expected, e := expectedChecksum(client, exe, sum)
	if e != nil {
		return e
	}
	work, e := os.MkdirTemp(filepath.Dir(target), ".chatgpt-update-stage-")
	if e != nil {
		return e
	}
	handedOff := false
	defer func() {
		if !handedOff {
			os.RemoveAll(work)
		}
	}()
	candidate := filepath.Join(work, exeAsset)
	if e := downloadExe(client, exe, expected, candidate); e != nil {
		return e
	}
	fmt.Println("[OK] Download size, SHA-256 and Windows x64 executable format verified.")
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return errors.New("LOCALAPPDATA is not set")
	}
	logs := filepath.Join(local, "ChatGPTUpdater", "Logs")
	if e := os.MkdirAll(logs, 0700); e != nil {
		return e
	}
	id := uniqueName()
	plan := replacementPlan{os.Getpid(), target, candidate, expected, strings.TrimPrefix(release.Tag, "v"), target + "." + id + ".previous", filepath.Join(logs, "self-update-"+id+".log")}
	b, e := json.MarshalIndent(plan, "", "  ")
	if e != nil {
		return e
	}
	planPath := filepath.Join(work, "replacement.json")
	if e := os.WriteFile(planPath, b, 0600); e != nil {
		return e
	}
	if e := startReplacementHelper(planPath); e != nil {
		return e
	}
	handedOff = true
	fmt.Println("[>>] Finishing in a new update window. This updater is exiting so its EXE can be replaced.")
	fmt.Println("Log:", plan.Log)
	fmt.Println("After completion, run chatgpt-update again. A .previous copy is retained for rollback.")
	return errUpdating
}
