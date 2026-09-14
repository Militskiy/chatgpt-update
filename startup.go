package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const startupCheckTimeout = 5 * time.Second

func shouldCheckAtStartup(args []string, opts uiOptions, interactive bool) bool {
	return interactive && !opts.SkipUpdateCheck && !enabledEnvironment("CI") &&
		(len(args) == 0 || (len(args) == 1 && args[0] == "menu"))
}

// Metadata only. Timeout includes headers, redirects AND response-body reads.
// No installer is downloaded and no persistent setting/file is written here.
func fetchLatestUpdater(client *http.Client, url string) (releaseInfo, error) {
	var release releaseInfo
	response, err := httpGet(client, url)
	if err != nil {
		return release, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil {
		return release, err
	}
	if len(data) > 2<<20 {
		return release, errors.New("release metadata exceeds the size limit")
	}
	if err = json.Unmarshal(data, &release); err != nil {
		return release, err
	}
	if release.Draft || release.Prerelease {
		return release, errors.New("latest release is not a published normal-channel release")
	}
	if _, err = parseVersion(release.Tag); err != nil {
		return release, err
	}
	return release, nil
}
func latestUpdaterWithTimeout(timeout time.Duration) (releaseInfo, error) {
	client := newHTTPClient()
	client.Timeout = timeout
	return fetchLatestUpdater(client, releaseAPI)
}

// Injected boundaries let tests prove no download/install happens after N,
// timeout, malformed metadata, or an old release; no live network is needed.
func startupUpdateCheck(fetch func() (releaseInfo, error), consent func(string) bool, install func(releaseInfo) error) error {
	stop := startActivity("Checking for an updater update (up to 5s)")
	release, err := fetch()
	if err != nil {
		stop("WARN")
		uiStatus("WARN", "Startup check unavailable; opening the menu. "+oneLine(err.Error(), 150))
		return nil
	}
	newer, err := newerVersion(release.Tag, version)
	if err != nil || release.Draft || release.Prerelease {
		stop("WARN")
		uiStatus("WARN", "Unusable release metadata; opening the menu without updating.")
		return nil
	}
	if !newer {
		stop("OK")
		uiStatus("OK", "No newer updater release. Installed: "+version)
		return nil
	}
	if _, _, err = choosePortable(release); err != nil {
		stop("WARN")
		uiStatus("WARN", "New release is not ready: "+oneLine(err.Error(), 150)+". Opening the menu.")
		return nil
	}
	stop("OK")
	fmt.Println(paint(yellow, "NEW UPDATER VERSION: "+version+" -> "+release.Tag))
	fmt.Println("Source: Militskiy/chatgpt-update GitHub Releases. This updates the utility, not ChatGPT.")
	fmt.Println("Unsigned package; normal-channel delivery is not a security clearance.")
	if !consent("Install this updater update now?") {
		uiStatus("SKIP", "Update postponed. Choose 4 later; no installer was downloaded.")
		return nil
	}
	return install(release)
}
func runStartupCheck() error {
	return startupUpdateCheck(
		func() (releaseInfo, error) { return latestUpdaterWithTimeout(startupCheckTimeout) },
		confirm,
		func(release releaseInfo) error {
			unlock, err := acquireOperationLock()
			if err != nil {
				return err
			}
			defer unlock()
			return installUpdaterRelease(release, true)
		},
	)
}
