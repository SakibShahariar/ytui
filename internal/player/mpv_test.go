package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestGetPropertyMatchesItsRequestAndSkipsEvents(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	p := &Player{conn: client, reader: bufio.NewReader(client)}
	serverErr := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(server)
		for i := 0; i < 2; i++ {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				serverErr <- err
				return
			}
			var request struct {
				Command   []interface{} `json:"command"`
				RequestID uint64        `json:"request_id"`
			}
			if err := json.Unmarshal(line, &request); err != nil {
				serverErr <- err
				return
			}
			if len(request.Command) != 2 {
				serverErr <- fmt.Errorf("unexpected command: %#v", request.Command)
				return
			}
			property, ok := request.Command[1].(string)
			if !ok {
				serverErr <- fmt.Errorf("property name is not a string")
				return
			}
			// mpv can emit events on the same connection before a command reply.
			if _, err := fmt.Fprintln(server, `{"event":"file-loaded"}`); err != nil {
				serverErr <- err
				return
			}
			response, err := json.Marshal(map[string]interface{}{
				"request_id": request.RequestID,
				"error":      "success",
				"data":       property,
			})
			if err != nil {
				serverErr <- err
				return
			}
			if _, err := fmt.Fprintln(server, string(response)); err != nil {
				serverErr <- err
				return
			}
		}
		serverErr <- nil
	}()

	var wg sync.WaitGroup
	results := make(chan string, 2)
	errors := make(chan error, 2)
	for _, name := range []string{"duration", "time-pos"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			value, err := p.GetProperty(name)
			if err != nil {
				errors <- err
				return
			}
			got, ok := value.(string)
			if !ok {
				errors <- fmt.Errorf("property %q returned %T", name, value)
				return
			}
			results <- got
		}(name)
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Errorf("GetProperty() error = %v", err)
	}
	got := map[string]bool{}
	for value := range results {
		got[value] = true
	}
	if !got["duration"] || !got["time-pos"] {
		t.Errorf("property results = %#v, want duration and time-pos", got)
	}
	if err := <-serverErr; err != nil {
		t.Errorf("fake mpv server error = %v", err)
	}
}


func TestWaitCanBeCalledConcurrently(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &Player{cmd: cmd}
	const callers = 8
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- p.Wait()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Wait() error = %v", err)
		}
	}
}

func TestWaitRemovesSocketPath(t *testing.T) {
	// Re-execute only this test as a short-lived child process. This exercises
	// Wait's real process lifecycle without requiring mpv to be installed.
	if os.Getenv("YTUI_WAIT_HELPER") == "1" {
		return
	}

	socketPath := filepath.Join(t.TempDir(), "mpv.sock")
	if err := os.WriteFile(socketPath, []byte("test socket placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestWaitRemovesSocketPath$")
	cmd.Env = append(os.Environ(), "YTUI_WAIT_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &Player{cmd: cmd, socketPath: socketPath}
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket path still exists or could not be checked after Wait(): %v", err)
	}
}
