package cloudprovider

import (
	lqredis "bargain/liquefy/cache"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetUnavailableMarkets(t *testing.T) {
	conn := lqredis.GetConnection()
	defer func() {
		conn.Do("FLUSHALL")
		conn.Close()
	}()

	_, _ = conn.Do("SET", "unavailable|us-east-1a|d3.xlarge", "")
	markets := GetUnavailableMarkets()
	assert.Equal(
		t,
		markets["us-east-1a"]["d3.xlarge"],
		struct{}{},
	)

	_, _ = conn.Do("SET", "unavailable|us-east-1b|m3.medium", "")
	_, _ = conn.Do("SET", "unavailable|us-east-1c|c3.xlarge", "")

	markets = GetUnavailableMarkets()
	assert.Equal(
		t,
		struct{}{},
		markets["us-east-1b"]["m3.medium"],
	)
	assert.Equal(
		t,
		struct{}{},
		markets["us-east-1c"]["c3.xlarge"],
	)
}

func TestMarkMarketUnavailable(t *testing.T) {
	conn := lqredis.GetConnection()
	defer func() {
		conn.Do("FLUSHALL")
		conn.Close()
	}()

	MarkMarketUnavailable(AZ("us-east-1a"), InstanceType("d3.xlarge"))

	_, err := conn.Do("GET", "unavailable|us-east-1a|d3.xlarge")
	assert.Nil(t, err)
}

func TestKeyFromMarket(t *testing.T) {
	assert.Equal(
		t,
		"unavailable|us-east-1a|d3.xlarge",
		keyFromMarket("unavailable", "us-east-1a", "d3.xlarge"),
	)
}

func TestParseUnavailableKey(t *testing.T) {
	az, instanceType := parseUnavailableKey("unavailable|us-east-1a|d3.xlarge")
	assert.Equal(
		t,
		"us-east-1a",
		az,
	)
	assert.Equal(
		t,
		"d3.xlarge",
		instanceType,
	)
}
