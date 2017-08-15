package cache

import (
	log "github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"os"
)

var pool *redis.Pool

func init() {
	pool = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			redisURL := os.Getenv("REDIS_URL")
			if redisURL == "" {
				redisURL = "redis://localhost:6379/0" // See https://www.iana.org/assignments/uri-schemes/prov/redis
			}
			return redis.DialURL(redisURL)
		},
	}
}

// GetConnection returns a pooled redis connection to localhost:6379
// It is the application's responsibility to close this connection via conn.Close()
func GetConnection() redis.Conn {
	return pool.Get()
}

// GetMatchingKeys returns an iterable of all keys matching `pattern`
func GetMatchingKeys(conn redis.Conn, pattern string) []string {
	var (
		cursor int64
		items  []string
	)

	var results []string
	for {
		// First grab raw redis Values
		values, err := redis.Values(conn.Do("SCAN", cursor, "MATCH", pattern))
		if err != nil {
			log.Errorf("Failed to scan keys for pattern %s", pattern)
			return []string{}
		}

		// Scan them into list of []string
		values, err = redis.Scan(values, &cursor, &items)
		if err != nil {
			log.Errorf("Failed to scan keys for pattern %s", pattern)
			return []string{}
		}

		results = append(results, items...)
		if cursor == 0 {
			// nothing left to scan, stop iterating
			break
		}
	}

	return results
}
