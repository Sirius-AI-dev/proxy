package collector

import (
	"fmt"
	"github.com/unibackend/uniproxy/internal/config"
	"testing"
	"time"
)

func TestOverflowCollector(*testing.T) {
	c := NewCollector(config.Collector{
		BatchSize:  100,
		BufferSize: 1000,
	}, func(data []interface{}) {
		fmt.Println("PUSHED", len(data), "RECORDS")
		time.Sleep(100 * time.Millisecond)
	})

	for i := 0; i <= 1000; i++ {
		c.Collect(i)
	}
}
