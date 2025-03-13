package releaseinfo

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"context"
	"sync"
	"time"
)

var _ Provider = (*CachedProvider)(nil)

// RKC - 2nd level registry
// CachedProvider maintains a simple cache of release image info and only queries
// the embedded provider when there is no cache hit.
type CachedProvider struct {
	Cache map[string]*ReleaseImage
	Inner Provider
	mu    sync.Mutex

	once sync.Once
}

func (p *CachedProvider) Lookup(ctx context.Context, image string, pullSecret []byte) (releaseImage *ReleaseImage, err error) {

	logger := ctrl.LoggerFrom(ctx)

	// Purge the cache every once in a while as a simple leak mitigation
	p.once.Do(func() {
		go func() {
			t := time.NewTicker(30 * time.Minute)
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					p.mu.Lock()
					logger.Info("RKC - 2 - CachedProvider Cache CLEAR")
					p.Cache = map[string]*ReleaseImage{}
					p.mu.Unlock()
				}
			}
		}()
	})
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.Cache[image]
	logger.Info("RKC - 2 - CachedProvider Cache Lookup", image, ok)
	if ok {
		return entry, nil
	}

	entry, err = p.Inner.Lookup(ctx, image, pullSecret)
	logger.Info("RKC - 2 - CachedProvider Inner Lookup", image, err)
	if err != nil {
		return nil, err
	}
	logger.Info("RKC - 2 - CachedProvider Cached Image", image)
	p.Cache[image] = entry
	return entry, nil
}
