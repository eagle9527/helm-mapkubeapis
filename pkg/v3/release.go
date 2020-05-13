/*
Copyright

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

package v3

import (
	"fmt"
	"log"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"

	common "github.com/hickeyma/helm-mapkubeapis/pkg/common"
)

// MapReleaseWithUnSupportedAPIs checks the latest release version for any deprecated or removed APIs in its metadata
// If it finds any, it will create a new release version with the APIs mapped to the supported versions
func MapReleaseWithUnSupportedAPIs(mapOptions common.MapOptions) error {
	cfg, err := GetActionConfig(mapOptions.ReleaseNamespace, mapOptions.KubeConfig)
	if err != nil {
		return fmt.Errorf("Failed to get Helm action configuration due to the following error: %s", err)
	}

	var releaseName = mapOptions.ReleaseName
	log.Printf("Get release '%s' latest version.\n", releaseName)
	releaseToMap, err := getLatestRelease(releaseName, cfg)
	if err != nil {
		return fmt.Errorf("Failed to get release '%s' latest version due to the following error: %s", mapOptions.ReleaseName, err)
	}

	log.Printf("Check release '%s' for deprecated or removed APIs...\n", releaseName)
	var origManifest = releaseToMap.Manifest
	//kubeVersion := cfg.Capabilities.KubeVersion.Version
	modifiedManifest, err := common.ReplaceManifestUnSupportedAPIs(origManifest, mapOptions.MapFile, "")
	if err != nil {
		return err
	}
	log.Printf("Finished checking release '%s' for deprecated or removed APIs.\n", releaseName)
	if modifiedManifest == origManifest {
		log.Printf("Release '%s' has no deprecated or removed APIs.\n", releaseName)
		return nil
	}

	log.Printf("Deprecated or removed APIs exist, updating release: %s.\n", releaseName)
	if !mapOptions.DryRun {
		if err := updateRelease(releaseToMap, modifiedManifest, cfg); err != nil {
			return fmt.Errorf("Failed to update release '%s' due to the following error: %s", releaseName, err)
		}
		log.Printf("Release '%s' with deprecated or removed APIs updated successfully to new version.\n", releaseName)
	}

	return nil
}

func updateRelease(origRelease *release.Release, modifiedManifest string, cfg *action.Configuration) error {
	// Update current release version to be superseded
	log.Printf("Set status of release version '%s' to 'superseded'.\n", getReleaseVersionName(origRelease))
	origRelease.Info.Status = release.StatusSuperseded
	if err := cfg.Releases.Update(origRelease); err != nil {
		return fmt.Errorf("failed to update release version '%s': %s", getReleaseVersionName(origRelease), err)
	}
	log.Printf("Release version '%s' updated successfully.\n", getReleaseVersionName(origRelease))

	// Using a shallow copy of current release version to update the object with the modification
	// and then store this new version
	var newRelease = origRelease
	newRelease.Manifest = modifiedManifest
	newRelease.Info.Description = common.UpgradeDescription
	newRelease.Info.LastDeployed = cfg.Now()
	newRelease.Version = origRelease.Version + 1
	newRelease.Info.Status = release.StatusDeployed
	log.Printf("Add release version '%s' with updated supported APIs.\n", getReleaseVersionName(origRelease))
	if err := cfg.Releases.Create(newRelease); err != nil {
		return fmt.Errorf("failed to create new release version '%s': %s", getReleaseVersionName(origRelease), err)
	}
	log.Printf("Release version '%s' added successfully.\n", getReleaseVersionName(origRelease))
	return nil
}

func getLatestRelease(releaseName string, cfg *action.Configuration) (*release.Release, error) {
	return cfg.Releases.Last(releaseName)
}

func getReleaseVersionName(rel *release.Release) string {
	return fmt.Sprintf("%s.v%d", rel.Name, rel.Version)
}
