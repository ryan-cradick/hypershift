package releaseinfo

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/golang/groupcache/lru"
	"github.com/openshift/hypershift/support/releaseinfo/registryclient"
)

const (
	ReleaseImageStreamFile   = "release-manifests/image-references"
	ReleaseImageMetadataFile = "release-manifests/0000_50_installer_coreos-bootimages.yaml"
)

var _ Provider = (*RegistryClientProvider)(nil)

// RegistryClientProvider uses a registry client to directly stream image
// content and extract image metadata.
type RegistryClientProvider struct {
	Log logr.Logger
}

// RKC Cache
var releaseImageCache = lru.New(700)
var count = 0

func (p *RegistryClientProvider) Lookup(ctx context.Context, image string, pullSecret []byte) (releaseImage *ReleaseImage, err error) {

	// RKC - Add cache for release image
	count++
	if count > 500 {
		p.Log.Info(fmt.Sprintf("RKC - Registry Client Cache update: %d", releaseImageCache.Len()))
		count = 0
	}

	psString := string(pullSecret)
	key := image + psString
	release, exists := releaseImageCache.Get(key)
	if exists && release != nil {
		return release.(*ReleaseImage), nil
	}

	fileContents, err := registryclient.ExtractImageFiles(ctx, image, pullSecret, ReleaseImageStreamFile, ReleaseImageMetadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to extract release metadata: %w", err)
	}

	if _, ok := fileContents[ReleaseImageStreamFile]; !ok {
		return nil, fmt.Errorf("release image references file not found in release image %s", image)
	}
	imageStream, err := DeserializeImageStream(fileContents[ReleaseImageStreamFile])
	if err != nil {
		return nil, err
	}

	if _, ok := fileContents[ReleaseImageMetadataFile]; !ok {
		return nil, fmt.Errorf("release image metadata file not found in release image %s", image)
	}
	coreOSMeta, err := DeserializeImageMetadata(fileContents[ReleaseImageMetadataFile])
	if err != nil {
		return nil, err
	}

	releaseImage = &ReleaseImage{
		ImageStream:    imageStream,
		StreamMetadata: coreOSMeta,
	}

	releaseImageCache.Add(key, releaseImage)
	return releaseImage, nil
}
