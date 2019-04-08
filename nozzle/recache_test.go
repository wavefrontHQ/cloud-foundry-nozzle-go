package nozzle

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func checkEntry(t *testing.T, c *RandomEvictionCache, key string) {
	v, ok := c.Get(key)
	assert.True(t, ok)
	assert.Equal(t, key, v)
}

func TestOversizedCache(t *testing.T) {
	c := NewRandomEvictionCache(1000)
	for i := 0; i < 1000; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s, 1*time.Hour)
	}
	for i := 0; i < 1000; i++ {
		checkEntry(t, c, strconv.Itoa(i))
	}
}

func TestUndersizedCache(t *testing.T) {
	c := NewRandomEvictionCache(1000)
	for i := 0; i < 2000; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s, 1*time.Hour)
	}
	hits := 0
	for i := 0; i < 2000; i++ {
		_, ok := c.Get(strconv.Itoa(i))
		if ok {
			hits++
		}
	}
	assert.True(t, hits > 900, fmt.Sprintf("Not enough hits: %d", hits))
	assert.True(t, hits < 1100, fmt.Sprintf("Too many hits: %d", hits))
	assert.Equal(t, 1000, len(c.entries))
}

func TestExpiration2(t *testing.T) {
	c := NewRandomEvictionCache(500)

	for i := 0; i < 500; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s, 1*time.Microsecond)
	}
	time.Sleep(1 * time.Second) // Wait for entriex to expire
	for i := 500; i < 1000; i++ {
		s := strconv.Itoa(i)
		c.Set(s, s, 1*time.Hour)
	}

	for i := 500; i < 1000; i++ {
		checkEntry(t, c, strconv.Itoa(i))
	}
	for i := 0; i < 500; i++ {
		{
			s := strconv.Itoa(i)
			_, ok := c.Get(s)
			assert.False(t, ok, "Expired key was found")
		}
	}
}
