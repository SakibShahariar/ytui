// Package player launches mpv and talks to it over its JSON IPC socket,
// so the TUI can control playback (pause, seek, volume, quit) instead of
// just fire-and-forgetting a separate mpv window.
//
// mpv has a built-in yt-dlp hook (ytdl_hook.lua) that resolves YouTube
// URLs itself, so we just hand it the watch-page URL directly — no need
// to pre-resolve a stream URL ourselves.
package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

// Player wraps a running mpv process and its IPC connection.
type Player struct {
	cmd        *exec.Cmd
	conn       net.Conn
	socketPath string
}

// mpvCommand is the shape mpv's IPC protocol expects for sending commands.
type mpvCommand struct {
	Command []interface{} `json:"command"`
}

// Play launches mpv pointed at videoURL. audioOnly strips video for a
// lighter, music-mode playback (handy over SSH/low bandwidth).
func Play(videoURL string, audioOnly bool) (*Player, error) {
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil, fmt.Errorf("mpv not found on PATH — install it with: sudo dnf install mpv")
	}

	// Unique per call (PID + nanosecond timestamp), not a fixed shared
	// path — a fixed path meant two Play() calls close together (e.g.
	// starting a new video right as the old one is being stopped) could
	// race for the same socket file before the old mpv fully released it,
	// causing playback to intermittently just silently fail to connect.
	socketPath := fmt.Sprintf("/tmp/ytui-mpv-%d-%d.sock", os.Getpid(), time.Now().UnixNano())
	_ = os.Remove(socketPath) // belt-and-suspenders; shouldn't exist given the above

	args := []string{
		"--no-terminal",
		"--input-ipc-server=" + socketPath,
		"--ytdl-format=bestvideo+bestaudio/best",
	}
	if audioOnly {
		args = append(args, "--no-video")
	}
	args = append(args, videoURL)

	cmd := exec.Command("mpv", args...)
	// Detach mpv's own stdout/stderr so it doesn't corrupt our TUI's screen.
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mpv: %w", err)
	}

	// mpv needs a beat to create the IPC socket after process start.
	var conn net.Conn
	var err error
	for i := 0; i < 20; i++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		// mpv is still playing even without IPC control, so don't kill it —
		// just report degraded (no in-TUI control) mode to the caller.
		return &Player{cmd: cmd, socketPath: socketPath}, fmt.Errorf("mpv started but IPC connect failed (no in-TUI control): %w", err)
	}

	return &Player{cmd: cmd, conn: conn, socketPath: socketPath}, nil
}

// sendCommand sends a raw mpv IPC command, e.g. []interface{}{"set_property", "pause", true}.
func (p *Player) sendCommand(args ...interface{}) error {
	if p.conn == nil {
		return fmt.Errorf("no IPC connection to mpv")
	}
	payload, err := json.Marshal(mpvCommand{Command: args})
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = p.conn.Write(payload)
	return err
}

// TogglePause toggles playback.
func (p *Player) TogglePause() error {
	return p.sendCommand("cycle", "pause")
}

// SeekRelative seeks forward/backward by the given number of seconds
// (negative rewinds).
func (p *Player) SeekRelative(seconds int) error {
	return p.sendCommand("seek", seconds, "relative")
}

// SeekAbsolute jumps to an absolute position in seconds — used to resume a
// video where playback last left off.
func (p *Player) SeekAbsolute(seconds float64) error {
	return p.sendCommand("seek", seconds, "absolute")
}

// Position returns the current playback position in seconds. Used to save
// a resume point when the person stops or quits mid-video.
func (p *Player) Position() (float64, error) {
	data, err := p.GetProperty("time-pos")
	if err != nil {
		return 0, err
	}
	pos, ok := data.(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected time-pos type from mpv")
	}
	return pos, nil
}

// VolumeAdjust changes volume by delta percentage points.
func (p *Player) VolumeAdjust(delta int) error {
	return p.sendCommand("add", "volume", delta)
}

// Duration returns the total length of the current file in seconds.
func (p *Player) Duration() (float64, error) {
	data, err := p.GetProperty("duration")
	if err != nil {
		return 0, err
	}
	d, ok := data.(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected duration type from mpv")
	}
	return d, nil
}

// Volume returns the current volume level (0-100, can exceed 100 if
// amplified above the default).
func (p *Player) Volume() (float64, error) {
	data, err := p.GetProperty("volume")
	if err != nil {
		return 0, err
	}
	v, ok := data.(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected volume type from mpv")
	}
	return v, nil
}

// IsPaused reports whether playback is currently paused.
func (p *Player) IsPaused() (bool, error) {
	data, err := p.GetProperty("pause")
	if err != nil {
		return false, err
	}
	paused, ok := data.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected pause type from mpv")
	}
	return paused, nil
}

// Quit stops mpv and cleans up the IPC connection/socket.
func (p *Player) Quit() error {
	if p.conn != nil {
		_ = p.sendCommand("quit")
		_ = p.conn.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		// Give mpv a moment to exit cleanly before hard-killing.
		done := make(chan error, 1)
		go func() { done <- p.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = p.cmd.Process.Kill()
		}
	}
	if p.socketPath != "" {
		_ = os.Remove(p.socketPath)
	}
	return nil
}

// Wait blocks until mpv exits (e.g. user pressed 'q' inside mpv itself,
// or the video finished). Useful when running without IPC control.
func (p *Player) Wait() error {
	if p.cmd == nil {
		return nil
	}
	return p.cmd.Wait()
}

// GetProperty queries an mpv property (e.g. "pause", "time-pos", "duration")
// and returns the raw JSON response reader for the caller to decode.
func (p *Player) GetProperty(name string) (interface{}, error) {
	if p.conn == nil {
		return nil, fmt.Errorf("no IPC connection to mpv")
	}
	req := map[string]interface{}{
		"command": []interface{}{"get_property", name},
	}
	payload, _ := json.Marshal(req)
	payload = append(payload, '\n')
	if _, err := p.conn.Write(payload); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(p.conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data  interface{} `json:"data"`
		Error string      `json:"error"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "success" {
		return nil, fmt.Errorf("mpv error: %s", resp.Error)
	}
	return resp.Data, nil
}
