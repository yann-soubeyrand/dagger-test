package main

import (
	"context"
	"fmt"
)

const (
	UtilityImageRegistry   string = "docker.io"
	UtilityImageRepository string = "busybox"
	UtilityImageTag        string = "1.36.1"
	UtilityImageDigest     string = "sha256:ba76950ac9eaa407512c9d859cea48114eeff8a6f12ebaa5d32ce79d4a017dd8"
)

type Geoip struct {
	LicenseKey *Secret
}

func New(ctx context.Context, licenseKey *Secret) (*Geoip, error) {
	geoip := &Geoip{
		LicenseKey: licenseKey,
	}

	return geoip, nil
}

func (geoip *Geoip) Database(ctx context.Context, editionID string) (*File, error) {
	licenseKey, err := geoip.LicenseKey.Plaintext(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get license key: %w", err)
	}

	downloadURL := fmt.Sprintf("https://download.maxmind.com/app/geoip_download?edition_id=%s&license_key=%s&suffix=tar.gz", editionID, licenseKey)

	tarballName := editionID + ".tar.gz"
	checksumsName := tarballName + ".sha256"

	tarball := dag.HTTP(downloadURL)
	checksums := dag.HTTP(downloadURL + ".sha256")

	container := dag.Container().
		From(UtilityImageRegistry+"/"+UtilityImageRepository+":"+UtilityImageTag+"@"+UtilityImageDigest).
		WithWorkdir("/home").
		WithMountedFile(tarballName, tarball).
		WithMountedFile(checksumsName, checksums).
		WithEntrypoint([]string{"sh", "-c"}).
		WithExec([]string{"sed --regexp-extended 's/\\S+$/" + tarballName + "/' " + checksumsName + " | sha256sum -c"}).
		WithExec([]string{"tar --extract --strip-components 1 --file " + tarballName})

	file := container.
		File(editionID + ".mmdb")

	return file, nil
}

func (geoip *Geoip) Directory(ctx context.Context, editionIDs []string, prefix Optional[string]) (*Directory, error) {
	_prefix := prefix.GetOr("/usr/local")

	directory := dag.Directory()

	for _, editionID := range editionIDs {
		database, err := geoip.Database(ctx, editionID)

		if err != nil {
			return nil, fmt.Errorf("failed to get database %q: %w", editionID, err)
		}

		directory = directory.
			WithFile(_prefix+"/share/GeoIP/"+editionID+".mmdb", database)
	}

	return directory, nil
}
