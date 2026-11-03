package github

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"pier/internal/engine/log"
)

func TestBackgroundDownloadProgressIsVisibleAndThrottled(t *testing.T) {
	var output bytes.Buffer
	log.SetOutput(&output)
	defer log.ResetOutput()
	p := newProgressReader(strings.NewReader(""), 100, 0, nil, 0)
	p.isTTY = false
	p.current = 50
	p.lastUpdate = time.Now().Add(-3 * time.Second)
	p.render()
	if !strings.Contains(output.String(), "50.0%") {
		t.Fatalf("background download progress missing: %s", output.String())
	}
	size := output.Len()
	p.render()
	if output.Len() != size {
		t.Fatal("progress was not throttled")
	}
	p.current = 100
	p.render()
	if !strings.Contains(output.String(), "100.0%") {
		t.Fatal("completion should be emitted immediately")
	}
}
