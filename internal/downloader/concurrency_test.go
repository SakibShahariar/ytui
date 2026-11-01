package downloader

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestQueueCountsAndPendingOrder(t *testing.T) {
	m := &Manager{
		byID: make(map[string]*Download),
		wake: make(chan struct{}, 1),
		maxConcurrent: 2,
	}
	for _, id := range []string{"first", "second", "third"} {
		m.Enqueue(id, id, "", "https://example.invalid/"+id, Options{})
	}
	active, queued, limit := m.QueueCounts()
	if active != 0 || queued != 3 || limit != 2 {
		t.Fatalf("QueueCounts() = (%d, %d, %d), want (0, 3, 2)", active, queued, limit)
	}
	if got := strings.Join(m.pending, ","); got != "first,second,third" {
		t.Fatalf("pending order = %q, want FIFO order", got)
	}
	if !m.MoveQueued("third", -1) {
		t.Fatal("MoveQueued() did not move pending item")
	}
	if got := strings.Join(m.pending, ","); got != "first,third,second" {
		t.Fatalf("pending order after move = %q, want first,third,second", got)
	}
	if got := strings.Join([]string{m.List()[0].VideoID, m.List()[1].VideoID, m.List()[2].VideoID}, ","); got != "first,third,second" {
		t.Fatalf("visible order = %q, want pending order", got)
	}
}

func TestNewWithConcurrencyRejectsOutOfRangeLimits(t *testing.T) {
	for _, limit := range []int{-1, 11} {
		if manager, err := NewWithConcurrency(t.TempDir(), limit); err == nil {
			manager.Shutdown()
			t.Errorf("NewWithConcurrency(limit=%d) succeeded, want validation error", limit)
		}
	}
}

func TestYTDLPConcurrencyHelper(t *testing.T) {
	if os.Getenv("YTUI_TEST_CONCURRENCY_HELPER") != "1" {
		return
	}
	lockPath := os.Getenv("YTUI_TEST_LOCK")
	activePath := os.Getenv("YTUI_TEST_ACTIVE")
	peakPath := os.Getenv("YTUI_TEST_PEAK")
	lock := func(update func()) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			err := os.Mkdir(lockPath, 0o700)
			if err == nil { break }
			if !os.IsExist(err) { t.Fatalf("acquire helper lock: %v", err) }
			if time.Now().After(deadline) { t.Fatal("timed out acquiring helper lock") }
			time.Sleep(time.Millisecond)
		}
		defer os.Remove(lockPath)
		update()
	}
	lock(func() {
		active := 0
		if data, err := os.ReadFile(activePath); err == nil {
			active, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		active++
		if err := os.WriteFile(activePath, []byte(strconv.Itoa(active)), 0o600); err != nil {
			t.Fatalf("write active process count: %v", err)
		}
		peak := 0
		if data, err := os.ReadFile(peakPath); err == nil {
			peak, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		if active > peak {
			if err := os.WriteFile(peakPath, []byte(strconv.Itoa(active)), 0o600); err != nil {
				t.Fatalf("write peak process count: %v", err)
			}
		}
	})
	time.Sleep(300 * time.Millisecond)
	lock(func() {
		data, err := os.ReadFile(activePath)
		if err != nil { t.Fatalf("read active process count: %v", err) }
		active, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil { t.Fatalf("parse active process count: %v", err) }
		active--
		if err := os.WriteFile(activePath, []byte(strconv.Itoa(active)), 0o600); err != nil {
			t.Fatalf("write active process count: %v", err)
		}
	})
}

func TestConcurrentDownloadsRespectConfiguredLimit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	binDir := t.TempDir()
	runDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	script := "#!/bin/sh\\nexec \"$YTUI_TEST_BINARY\" -test.run=^TestYTDLPConcurrencyHelper$\\n"
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("YTUI_TEST_BINARY", os.Args[0])
	t.Setenv("YTUI_TEST_CONCURRENCY_HELPER", "1")
	t.Setenv("YTUI_TEST_LOCK", filepath.Join(runDir, "lock"))
	activePath := filepath.Join(runDir, "active")
	peakPath := filepath.Join(runDir, "peak")
	t.Setenv("YTUI_TEST_ACTIVE", activePath)
	t.Setenv("YTUI_TEST_PEAK", peakPath)

	manager, err := NewWithConcurrency(t.TempDir(), 2)
	if err != nil {
		t.Fatalf("NewWithConcurrency() error = %v", err)
	}
	for i := 0; i < 5; i++ {
		id := "video-" + strconv.Itoa(i)
		manager.Enqueue(id, id, "", "https://example.invalid/"+id, Options{})
	}
	deadline := time.Now().Add(8 * time.Second)
	for manager.Active() {
		if time.Now().After(deadline) {
			manager.Shutdown()
			t.Fatalf("downloads did not finish before timeout: %#v", manager.List())
		}
		time.Sleep(10 * time.Millisecond)
	}
	manager.Shutdown()
	peakData, err := os.ReadFile(peakPath)
	if err != nil {
		t.Fatalf("read peak concurrency: %v", err)
	}
	peak, err := strconv.Atoi(strings.TrimSpace(string(peakData)))
	if err != nil {
		t.Fatalf("parse peak concurrency %q: %v", peakData, err)
	}
	if peak != 2 {
		t.Fatalf("peak concurrent processes = %d, want exactly 2", peak)
	}
	for _, item := range manager.List() {
		if item.Status != StatusDone {
			t.Errorf("download %q status = %v, want done", item.VideoID, item.Status)
		}
	}
}

func TestParallelShutdownPreservesActiveAndQueuedOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp fixture uses a POSIX shell script")
	}
	binDir := t.TempDir()
	destDir := t.TempDir()
	partialDir := t.TempDir()
	fakeYTDLP := filepath.Join(binDir, "yt-dlp")
	script := `#!/bin/sh
set -eu
last=""
for arg do last="$arg"; done
id="${last##*/}"
: > "$YTUI_TEST_PARTIAL_DIR/$id.part"
while :; do sleep 1; done
`
	if err := os.WriteFile(fakeYTDLP, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("YTUI_TEST_PARTIAL_DIR", partialDir)

	manager, err := NewWithConcurrency(destDir, 2)
	if err != nil {
		t.Fatalf("NewWithConcurrency() error = %v", err)
	}
	for _, id := range []string{"first", "second", "third"} {
		manager.Enqueue(id, id, "", "https://example.invalid/"+id, Options{})
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		items := manager.List()
		active := 0
		for _, item := range items {
			if item.Status == StatusDownloading {
				active++
			}
		}
		_, firstErr := os.Stat(filepath.Join(partialDir, "first.part"))
		_, secondErr := os.Stat(filepath.Join(partialDir, "second.part"))
		if active == 2 && firstErr == nil && secondErr == nil {
			break
		}
		if time.Now().After(deadline) {
			manager.Shutdown()
			t.Fatalf("two parallel downloads did not start: %#v", items)
		}
		time.Sleep(10 * time.Millisecond)
	}
	manager.Shutdown()

	items := manager.List()
	wantIDs := []string{"first", "second", "third"}
	for i, wantID := range wantIDs {
		if items[i].VideoID != wantID || items[i].Status != StatusQueued {
			t.Fatalf("shutdown item %d = (%q, %v), want (%q, queued)", i, items[i].VideoID, items[i].Status, wantID)
		}
	}
	for _, id := range wantIDs[:2] {
		if _, err := os.Stat(filepath.Join(partialDir, id+".part")); err != nil {
			t.Errorf("shutdown removed %s partial file: %v", id, err)
		}
	}

	restored := &Manager{byID: make(map[string]*Download), queueFile: filepath.Join(destDir, ".ytui-queue.json")}
	if err := restored.loadQueue(); err != nil {
		t.Fatalf("restore queue after parallel shutdown: %v", err)
	}
	if got := strings.Join(restored.order, ","); got != "first,second,third" {
		t.Fatalf("restored queue order = %q, want first,second,third", got)
	}
}
