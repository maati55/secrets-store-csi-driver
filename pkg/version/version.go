/*
Copyright 2019 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package version

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/blang/semver"
	log "github.com/sirupsen/logrus"
)

// providerVersion holds current provider version
type providerVersion struct {
	// Version is the current provider version
	Version string `json:"version"`
	// BuildDate is the date provider binary was built
	BuildDate string `json:"buildDate"`
	// MinDriverVersion is minimum driver version the provider works with
	// this can be used later for bidirectional compatibility checks between driver-provider
	MinDriverVersion string `json:"minDriverVersion"`
}

// IsProviderCompatible checks if the provider version is compatible with
// current driver version.
func IsProviderCompatible(ctx context.Context, provider string, minProviderVersion string) (bool, error) {
	// get current provider version
	currProviderVersion, err := getProviderVersion(ctx, provider)
	if err != nil {
		return false, err
	}
	// check with normalized versions
	return isProviderCompatible(normalizeVersion(currProviderVersion), normalizeVersion(minProviderVersion))
}

// GetMinimumProviderVersions creates a map with provider name and minimum version
// supported with this driver.
func GetMinimumProviderVersions(minProviderVersions string) (map[string]string, error) {
	providerVersionMap := make(map[string]string)

	if minProviderVersions == "" {
		return providerVersionMap, nil
	}

	// splitting on , delimiter will result in array of provider=value string
	providers := strings.Split(minProviderVersions, ",")
	for _, p := range providers {
		p = strings.TrimSpace(p)
		pv := strings.Split(p, "=")

		if len(pv) != 2 {
			return providerVersionMap, fmt.Errorf("min provider version not defined in expected format, got %+v", pv)
		}

		provider := strings.TrimSpace(pv[0])
		version := strings.TrimSpace(pv[1])

		// check if in expected format provider=version
		if len(provider) == 0 || len(version) == 0 {
			return providerVersionMap, fmt.Errorf("min provider version not defined in expected format provider=version, got provider %s version %s", provider, version)
		}
		// check if duplicate provider name
		if v, exists := providerVersionMap[provider]; exists {
			return providerVersionMap, fmt.Errorf("duplicate versions defined for %s provider, versions: [%s, %s]", provider, v, version)
		}
		// check if provided version is a valid semver
		if err := isValidSemver(version); err != nil {
			return providerVersionMap, fmt.Errorf("minimum %s provider version %s is not a valid semver, error %+v", provider, version, err)
		}

		providerVersionMap[provider] = version
	}

	log.Debugf("Minimum supported provider versions: %v", providerVersionMap)
	return providerVersionMap, nil
}

func getProviderVersion(ctx context.Context, providerName string) (string, error) {
	cmd := exec.CommandContext(ctx, providerName, "--version")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stderr, cmd.Stdout = stderr, stdout

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error getting current provider version for %s, err: %v, output: %v", providerName, err, stderr.String())
	}
	var pv providerVersion
	if err := json.Unmarshal(stdout.Bytes(), &pv); err != nil {
		return "", fmt.Errorf("error unmarshalling provider version %v", err)
	}

	log.Debugf("provider: %s, version %s, build date: %s", providerName, pv.Version, pv.BuildDate)
	return pv.Version, nil
}

func isProviderCompatible(currVersion, minVersion string) (bool, error) {
	currV, err := semver.Make(currVersion)
	if err != nil {
		return false, err
	}
	minV, err := semver.Make(minVersion)
	if err != nil {
		return false, err
	}
	return currV.Compare(minV) >= 0, nil
}

func isValidSemver(version string) error {
	_, err := semver.Make(version)
	return err
}

func normalizeVersion(version string) string {
	// driver currently uses prefix in version
	// no checks are currently performed using driver version, but
	// will be done in the future for bi-directional version validation.
	return strings.TrimPrefix(version, "v")
}
