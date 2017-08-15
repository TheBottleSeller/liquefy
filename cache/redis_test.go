package cache

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetMatchingKeys(t *testing.T) {
	conn := GetConnection()
	defer func() {
		conn.Do("FLUSHALL")
		conn.Close()
	}()

	_, _ = conn.Do("SET", "unavailable|us-east-1b|m3.medium", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1c|c3.xlarge", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1a|m3.medium", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1d|m3.medium", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1b|c3.xlarge", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1a|c3.xlarge", nil)

	assert.Equal(
		t,
		6,
		len(GetMatchingKeys(conn, "unavailable|*")),
	)

	// Force pagination of results since default COUNT is 10
	_, _ = conn.Do("SET", "unavailable|us-east-1a|t1.micro", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1b|t1.micro", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1c|t1.micro", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1d|t1.micro", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1a|t2.micro", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1b|t2.micro", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1c|t2.micro", nil)
	_, _ = conn.Do("SET", "unavailable|us-east-1d|t2.micro", nil)

	assert.Equal(
		t,
		14,
		len(GetMatchingKeys(conn, "unavailable|*")),
	)

	// Check non existent pattern
	assert.Empty(
		t,
		GetMatchingKeys(conn, "nonexistent|*"),
	)
}
