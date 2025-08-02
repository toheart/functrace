package trace

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultCacheExpiration = 5 * time.Minute
	defaultCheckInterval   = 10 * time.Second
)

// TTLCacheManager manages the lifecycle of cached parameters using a time-to-live (TTL) mechanism.
type TTLCacheManager struct {
	paramTTL       sync.Map
	stopCh         chan struct{}
	wg             sync.WaitGroup
	log            *logrus.Logger
	expirationTime time.Duration
	checkInterval  time.Duration
}

// NewTTLCacheManager creates and returns a new TTLCacheManager.
func NewTTLCacheManager(log *logrus.Logger) *TTLCacheManager {
	return &TTLCacheManager{
		stopCh:         make(chan struct{}),
		log:            log,
		expirationTime: defaultCacheExpiration,
		checkInterval:  defaultCheckInterval,
	}
}

// Start begins the background goroutine for cleaning up expired cache entries.
func (tm *TTLCacheManager) Start() {
	tm.wg.Add(1)
	go tm.runCleanupLoop()
	tm.log.Info("TTLCacheManager started.")
}

// Stop signals the cleanup goroutine to terminate and waits for it to finish.
func (tm *TTLCacheManager) Stop() {
	close(tm.stopCh)
	tm.wg.Wait()
	tm.log.Info("TTLCacheManager stopped.")
}

// Update refreshes the TTL for a given cache address.
func (tm *TTLCacheManager) Update(addr string) {
	if addr != "" {
		tm.paramTTL.Store(addr, time.Now())
	}
}

// runCleanupLoop is the main loop for the cleanup goroutine.
func (tm *TTLCacheManager) runCleanupLoop() {
	defer tm.wg.Done()
	ticker := time.NewTicker(tm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.cleanupExpiredEntries()
		case <-tm.stopCh:
			return
		}
	}
}

// cleanupExpiredEntries iterates through the cache and removes entries that have expired.
func (tm *TTLCacheManager) cleanupExpiredEntries() {
	now := time.Now()
	cleanedCount := 0

	tm.paramTTL.Range(func(key, value interface{}) bool {
		addr := key.(string)
		lastAccessTime := value.(time.Time)

		if now.Sub(lastAccessTime) > tm.expirationTime {
			tm.paramTTL.Delete(addr)

			// Since the cache entry has expired in memory, we also remove it from the persistent storage.
			if err := repositoryFactory.GetParamRepository().DeleteParamCacheByAddr(addr); err != nil {
				tm.log.WithFields(logrus.Fields{"error": err, "addr": addr}).Error("failed to delete param cache from database")
			} else {
				cleanedCount++
			}
		}
		return true
	})

	if cleanedCount > 0 {
		tm.log.WithFields(logrus.Fields{"count": cleanedCount}).Info("cleaned up expired param cache entries")
	}
}
