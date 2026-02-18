// Package dui provides a Go client for communicating with DUI (Diplomacy
// Universal Interface) engines. It manages the engine subprocess, handles
// the protocol handshake, and provides methods for sending commands and
// parsing responses.
//
// Inspired by github.com/freeeve/uci for chess UCI engines, adapted for
// Diplomacy concepts (DFEN positions, DSON orders, 7 powers).
package dui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Engine wraps a DUI-compatible engine subprocess. It manages the process
// lifecycle, sends commands via stdin, and reads responses from stdout.
type Engine struct {
	path string
	args []string

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner

	mu     sync.Mutex
	closed bool
	exited chan struct{}

	// Handshake results populated during Init.
	ID      EngineID
	Options []EngineOption
}

// NewEngine creates a new Engine pointing to the given binary path.
// The engine process is not started until Init is called.
func NewEngine(path string, args ...string) *Engine {
	return &Engine{
		path: path,
		args: args,
	}
}

// Init starts the engine subprocess and performs the DUI handshake
// (dui -> id/option/duiok, isready -> readyok). The provided context
// controls the overall timeout for the handshake.
func (e *Engine) Init(ctx context.Context) error {
	if err := e.start(ctx); err != nil {
		return fmt.Errorf("dui: start engine: %w", err)
	}

	if err := e.handshake(ctx); err != nil {
		e.Close()
		return fmt.Errorf("dui: handshake: %w", err)
	}

	return nil
}

// SetOption sends a "setoption" command to the engine.
func (e *Engine) SetOption(name, value string) {
	if value != "" {
		e.send(fmt.Sprintf("setoption name %s value %s", name, value))
	} else {
		e.send(fmt.Sprintf("setoption name %s", name))
	}
}

// IsReady sends "isready" and blocks until "readyok" is received or the
// context is canceled. Use this to synchronize after SetOption or Position.
func (e *Engine) IsReady(ctx context.Context) error {
	e.send("isready")
	return e.readUntil(ctx, "readyok")
}

// NewGame sends "newgame" to reset the engine's internal state.
func (e *Engine) NewGame() {
	e.send("newgame")
}

// Position sends a "position <dfen>" command to set the board state.
func (e *Engine) Position(dfen string) {
	e.send(fmt.Sprintf("position %s", dfen))
}

// SetPower sends a "setpower <power>" command to set the active power.
func (e *Engine) SetPower(power string) {
	e.send(fmt.Sprintf("setpower %s", power))
}

// Go sends a "go" command with the given parameters and reads the engine's
// response until "bestorders" is received. All "info" lines emitted during
// search are collected into the returned SearchResults.
//
// If the context is canceled before bestorders is received, a "stop" command
// is sent and the method waits briefly for the forced bestorders response.
func (e *Engine) Go(ctx context.Context, params GoParams) (*SearchResults, error) {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, fmt.Errorf("dui: engine is closed")
	}
	e.mu.Unlock()

	if !e.isAlive() {
		return nil, fmt.Errorf("dui: engine process is not running")
	}

	suffix := params.String()
	if suffix != "" {
		e.send("go " + suffix)
	} else {
		e.send("go")
	}

	return e.readSearchResults(ctx)
}

// Stop sends the "stop" command to interrupt the current search.
func (e *Engine) Stop() {
	e.send("stop")
}

// Quit sends "quit" to the engine. For full cleanup use Close instead.
func (e *Engine) Quit() {
	e.send("quit")
}

// Close sends "quit" to the engine and waits for process exit. If the
// process does not exit within 3 seconds, it is forcefully killed.
func (e *Engine) Close() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	if e.stdin != nil {
		fmt.Fprintf(e.stdin, "quit\n")
	}
	e.closed = true
	e.mu.Unlock()

	if e.stdin != nil {
		e.stdin.Close()
	}

	if e.exited != nil {
		select {
		case <-e.exited:
		case <-time.After(3 * time.Second):
			log.Printf("dui: engine did not exit within 3s, killing")
			if e.cmd != nil && e.cmd.Process != nil {
				e.cmd.Process.Kill()
			}
			<-e.exited
		}
	}
	return nil
}

// start launches the engine subprocess.
func (e *Engine) start(ctx context.Context) error {
	e.cmd = exec.CommandContext(ctx, e.path, e.args...)

	var err error
	e.stdin, err = e.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := e.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	e.scanner = bufio.NewScanner(stdout)
	e.exited = make(chan struct{})

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	go func() {
		e.cmd.Wait()
		close(e.exited)
	}()

	return nil
}

// handshake performs the DUI initialization sequence: sends "dui", reads
// id/option lines until "duiok", then sends "isready" and waits for "readyok".
func (e *Engine) handshake(ctx context.Context) error {
	e.send("dui")

	if err := e.readHandshake(ctx); err != nil {
		return fmt.Errorf("waiting for duiok: %w", err)
	}

	e.send("isready")
	if err := e.readUntil(ctx, "readyok"); err != nil {
		return fmt.Errorf("waiting for readyok: %w", err)
	}

	return nil
}

// readHandshake reads lines until "duiok", parsing id, option, and
// protocol_version lines along the way.
func (e *Engine) readHandshake(ctx context.Context) error {
	type result struct {
		err error
	}
	ch := make(chan result, 1)

	go func() {
		for e.scanner.Scan() {
			line := e.scanner.Text()

			switch {
			case strings.HasPrefix(line, "id name "):
				e.ID.Name = strings.TrimPrefix(line, "id name ")
			case strings.HasPrefix(line, "id author "):
				e.ID.Author = strings.TrimPrefix(line, "id author ")
			case strings.HasPrefix(line, "protocol_version "):
				fmt.Sscanf(strings.TrimPrefix(line, "protocol_version "), "%d", &e.ID.ProtocolVersion)
			case strings.HasPrefix(line, "option "):
				e.Options = append(e.Options, parseEngineOption(line))
			case line == "duiok":
				ch <- result{}
				return
			}
		}
		if err := e.scanner.Err(); err != nil {
			ch <- result{err: fmt.Errorf("scanner: %w", err)}
		} else {
			ch <- result{err: fmt.Errorf("engine closed stdout before duiok")}
		}
	}()

	select {
	case r := <-ch:
		return r.err
	case <-ctx.Done():
		return fmt.Errorf("context canceled: %w", ctx.Err())
	}
}

// readSearchResults reads lines from the engine during a "go" search,
// collecting info lines until "bestorders" is found. If the context is
// canceled, "stop" is sent and a grace period is given.
func (e *Engine) readSearchResults(ctx context.Context) (*SearchResults, error) {
	type result struct {
		sr  *SearchResults
		err error
	}

	ch := make(chan result, 1)
	go func() {
		sr := &SearchResults{}
		for e.scanner.Scan() {
			line := e.scanner.Text()
			if strings.HasPrefix(line, "bestorders ") {
				sr.BestOrders = strings.TrimPrefix(line, "bestorders ")
				ch <- result{sr: sr}
				return
			}
			if strings.HasPrefix(line, "info ") {
				sr.Infos = append(sr.Infos, parseInfo(line))
			}
		}
		if err := e.scanner.Err(); err != nil {
			ch <- result{err: fmt.Errorf("scanner: %w", err)}
		} else {
			ch <- result{err: fmt.Errorf("engine closed stdout unexpectedly")}
		}
	}()

	select {
	case r := <-ch:
		return r.sr, r.err
	case <-ctx.Done():
		e.send("stop")
		select {
		case r := <-ch:
			return r.sr, r.err
		case <-time.After(2 * time.Second):
			return nil, fmt.Errorf("dui: engine did not respond to stop within 2s")
		}
	}
}

// readUntil reads lines until the expected line is seen, ignoring others.
func (e *Engine) readUntil(ctx context.Context, expected string) error {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		for e.scanner.Scan() {
			line := e.scanner.Text()
			if line == expected {
				ch <- line
				return
			}
		}
		if err := e.scanner.Err(); err != nil {
			errCh <- err
		} else {
			errCh <- fmt.Errorf("engine closed stdout before sending %q", expected)
		}
	}()

	select {
	case <-ch:
		return nil
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("context canceled waiting for %q: %w", expected, ctx.Err())
	}
}

// send writes a command line to the engine's stdin.
func (e *Engine) send(line string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.stdin == nil {
		return
	}
	fmt.Fprintf(e.stdin, "%s\n", line)
}

// isAlive checks whether the engine process is still running.
func (e *Engine) isAlive() bool {
	if e.exited == nil {
		return false
	}
	select {
	case <-e.exited:
		return false
	default:
		return true
	}
}
