package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func validNewRelease() releaseInfo {
	return releaseInfo{Tag: "v99.0.0", Assets: []releaseAsset{
		{Name: portableAsset, URL: "https://github.com/Militskiy/chatgpt-update/releases/download/v99.0.0/" + portableAsset, State: "uploaded", Size: 2048},
	}}
}
func TestStartupCheckRoutes(t *testing.T) {
	t.Setenv("CI", "")
	for _, tc := range []struct {
		args                    []string
		interactive, skip, want bool
	}{
		{nil, true, false, true}, {[]string{"menu"}, true, false, true},
		{nil, false, false, false}, {nil, true, true, false},
		{[]string{"check"}, true, false, false}, {[]string{"backup"}, true, false, false},
		{[]string{"restore"}, true, false, false}, {[]string{"preview"}, true, false, false},
		{[]string{"--version"}, true, false, false}, {[]string{"--verify-package"}, true, false, false},
		{[]string{"--help"}, true, false, false}, {[]string{"self-update"}, true, false, false},
	} {
		if got := shouldCheckAtStartup(tc.args, uiOptions{SkipUpdateCheck: tc.skip}, tc.interactive); got != tc.want {
			t.Errorf("args=%v got %v", tc.args, got)
		}
	}
	t.Setenv("CI", "true")
	if shouldCheckAtStartup(nil, uiOptions{}, true) {
		t.Fatal("CI performed startup check")
	}
}
func TestStartupApprovalAndFallback(t *testing.T) {
	for _, name := range []string{"yes", "no", "offline", "equal", "older", "draft", "prerelease", "invalid", "bad-asset", "install-error"} {
		t.Run(name, func(t *testing.T) {
			calls, asked, installed := 0, 0, 0
			r := validNewRelease()
			switch name {
			case "equal":
				r.Tag = "v" + version
			case "older":
				r.Tag = "v0.0.1"
			case "draft":
				r.Draft = true
			case "prerelease":
				r.Prerelease = true
			case "invalid":
				r.Tag = "latest"
			case "bad-asset":
				r.Assets[0].URL = "https://example.com/evil.zip"
			}
			err := startupUpdateCheck(func() (releaseInfo, error) {
				calls++
				if name == "offline" {
					return r, errors.New("network unavailable")
				}
				return r, nil
			},
				func(prompt string) bool {
					asked++
					if !strings.Contains(prompt, "updater") {
						t.Error("ambiguous prompt")
					}
					return name != "no"
				},
				func(got releaseInfo) error {
					installed++
					if got.Tag != r.Tag {
						t.Error("changed selected release")
					}
					if name == "install-error" {
						return errors.New("blocked helper")
					}
					return errUpdating
				})
			wantPrompt := name == "yes" || name == "no" || name == "install-error"
			if calls != 1 || (asked == 1) != wantPrompt {
				t.Fatalf("fetch=%d prompt=%d", calls, asked)
			}
			if name == "yes" {
				if installed != 1 || !errors.Is(err, errUpdating) {
					t.Fatal("approved update not handed off")
				}
			} else if name == "install-error" {
				if installed != 1 || err == nil || errors.Is(err, errUpdating) {
					t.Fatal("failure was hidden")
				}
			} else if installed != 0 || err != nil {
				t.Fatalf("unexpected installation or fatal startup error: %d %v", installed, err)
			}
		})
	}
}
func TestStartupMetadataHTTP(t *testing.T) {
	for _, name := range []string{"valid", "404", "invalid", "draft", "slow-body", "oversized"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch name {
				case "404":
					w.WriteHeader(404)
				case "invalid":
					w.Write([]byte(`not json`))
				case "draft":
					w.Write([]byte(`{"tag_name":"v99.0.0","draft":true}`))
				case "oversized":
					w.Write([]byte(strings.Repeat(" ", (2<<20)+1)))
				case "slow-body":
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				default:
					w.Write([]byte(`{"tag_name":"v99.0.0"}`))
				}
			}))
			defer server.Close()
			client := newHTTPClient()
			client.Timeout = 150 * time.Millisecond
			start := time.Now()
			_, err := fetchLatestUpdater(client, server.URL)
			if (err == nil) != (name == "valid") {
				t.Fatalf("unexpected result %v", err)
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("metadata check blocked beyond timeout")
			}
		})
	}
}
