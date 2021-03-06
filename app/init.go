package app

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	anacrolixlog "github.com/anacrolix/log"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/mse"
	"github.com/boltdb/bolt"
	"golang.org/x/time/rate"

	"github.com/WinPooh32/peerstohttp/settings"
)

// https://gitlab.com/axet/libtorrent/-/blob/master/libtorrent.go
func limit(kbps int) *rate.Limiter {
	var l = rate.NewLimiter(rate.Inf, 0)

	if kbps > 0 {
		b := kbps
		if b < 16*1024 {
			b = 16 * 1024
		}
		l = rate.NewLimiter(rate.Limit(kbps), b)
	}

	return l
}

func p2p(service *settings.Settings, cwd string) (*torrent.Client, error) {
	var cfg *torrent.ClientConfig = torrent.NewDefaultClientConfig()

	// Bind port.
	cfg.ListenPort = *service.TorrPort

	// Download dir.
	cfg.DataDir = cwd

	// Rate limits.
	const kib = 1 << 10

	if *service.DownloadRate != 0 {
		cfg.DownloadRateLimiter = limit(*service.DownloadRate * kib)
	}

	if *service.UploadRate != 0 {
		cfg.UploadRateLimiter = limit(*service.UploadRate * kib)
	}

	// Connections limits.
	cfg.EstablishedConnsPerTorrent = *service.MaxConnections
	cfg.TorrentPeersLowWater = *service.MaxConnections

	// Discovery services.
	cfg.NoDHT = *service.NoDHT
	cfg.DisableUTP = *service.NoUTP
	cfg.DisableTCP = *service.NoTCP

	cfg.DisableIPv4 = *service.NoIPv4
	cfg.DisableIPv6 = *service.NoIPv6

	if *service.ProxyHTTP != "" {
		var u, err = url.Parse(*service.ProxyHTTP)
		if err != nil {
			return nil, fmt.Errorf("parse http proxy url: %w", err)
		}

		cfg.HTTPProxy = http.ProxyURL(u)
	}

	// Enable seeding.
	cfg.Seed = true

	// Header obfuscation.
	cfg.HeaderObfuscationPolicy = torrent.HeaderObfuscationPolicy{
		Preferred:        true,
		RequirePreferred: *service.ForceEncryption,
	}
	// Force encryption.
	if *service.ForceEncryption {
		cfg.CryptoProvides = mse.CryptoMethodRC4
	}

	cfg.DefaultRequestStrategy = torrent.RequestStrategyFastest()

	// Torrent debug.
	cfg.Debug = false

	if !*service.TorrentDebug {
		cfg.Logger = anacrolixlog.Discard
	}

	return torrent.NewClient(cfg)
}

func db(path string) (*bolt.DB, error) {
	var db, err = bolt.Open(path, 0600, &bolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	// Create buckets.
	err = db.Update(func(tx *bolt.Tx) error {
		var _, err = tx.CreateBucketIfNotExists([]byte(dbBucketInfo))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}
