package nozzle

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

type DummyPreloader struct {
	size int
	ai []AppInfo
}

func (d *DummyPreloader) GetAllApps() ([]AppInfo, error) {
	for i := 0; i < d.size; i++ {
		uuid, err  := uuid.NewRandom()
		if err != nil {
			return nil, err
		}
		s := strconv.Itoa(i)
		d.ai[i].Guid = uuid.String()
		d.ai[i].Name = "Name_" + s
		d.ai[i].Org = "Org_" + s
		d.ai[i].Space = "Space_" + s
	}
	return d.ai, nil
}

func TestOversizedPreloadedCache(t *testing.T) {
	pl := &DummyPreloader{ size:1000, ai: make([]AppInfo, 1000) }
	c := NewPreloadedCache(pl, 2000)
	for _, ai := range pl.ai {
		cai, ok := c.Get(ai.Guid)
		if ok {
			assert.Equal(t, ai.Guid, cai.(*AppInfo).Guid)
		}
	}
}

func TestUndersizedPreloadedCache(t *testing.T) {
	pl := &DummyPreloader{ size:1000, ai: make([]AppInfo, 1000) }
	c := NewPreloadedCache(pl, 200)
	for _, ai := range pl.ai {
		cai, ok := c.Get(ai.Guid)
		if ok {
			assert.Equal(t, ai.Guid, cai.(*AppInfo).Guid)
		}
	}
}