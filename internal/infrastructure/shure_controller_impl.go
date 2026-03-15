package infrastructure

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

// shureController is the concrete implementation of ShureController
type shureController struct {
	addr      string
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	isRunning bool
	events    chan interface{}
	done      chan struct{}
	wg        sync.WaitGroup
}

// NewShureController creates a new ShureController instance
func NewShureController(addr string) ShureController {
	if addr == "" {
		addr = "localhost:2202" // Default Shure Axient control port
	}
	return &shureController{
		addr:   addr,
		events: make(chan interface{}, 100),
		done:   make(chan struct{}),
	}
}

// Start begins the Shure controller
func (c *shureController) Start(ctx context.Context) error {
	if c.isRunning {
		return nil
	}

	// Establish TCP connection to Shure Axient system
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to Shure Axient at %s: %w", c.addr, err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	c.isRunning = true

	// Start goroutine to read events from Shure system
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.readEvents(ctx)
	}()

	// Start goroutine to handle connection lifecycle
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		<-ctx.Done()
		c.Stop(context.Background())
	}()

	return nil
}

// Stop halts the Shure controller
func (c *shureController) Stop(ctx context.Context) error {
	if !c.isRunning {
		return nil
	}

	c.isRunning = false
	close(c.done)

	// Close connection
	if c.conn != nil {
		c.conn.Close()
	}

	// Wait for goroutines to finish
	c.wg.Wait()

	// Close events channel
	close(c.events)

	return nil
}

// SendCommand sends a command to the Shure system
func (c *shureController) SendCommand(command interface{}) error {
	if !c.isRunning {
		return errors.New("controller not running")
	}

	if c.conn == nil {
		return errors.New("connection not established")
	}

	// Convert command to string
	var cmdStr string
	switch v := command.(type) {
	case string:
		cmdStr = v
	case fmt.Stringer:
		cmdStr = v.String()
	default:
		return fmt.Errorf("unsupported command type: %T", command)
	}

	// Ensure command ends with newline (required by Shure protocol)
	if !strings.HasSuffix(cmdStr, "\n") {
		cmdStr += "\n"
	}

	// Write command to Shure system
	_, err := c.writer.WriteString(cmdStr)
	if err != nil {
		return fmt.Errorf("failed to write command: %w", err)
	}

	// Flush to ensure command is sent
	err = c.writer.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

// ReceiveEvents returns a channel for receiving Shure events
func (c *shureController) ReceiveEvents() <-chan interface{} {
	return c.events
}

// readEvents continuously reads events from the Shure system
func (c *shureController) readEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
			// Read until '>' which is the end of a Shure TPCI message
			line, err := c.reader.ReadString('>')
			if err != nil {
				if !errors.Is(err, net.ErrClosed) && ctx.Err() == nil {
					slog.Error("Shure read error", "address", c.addr, "error", err)
				}
				return
			}

			// Clean up the message (Shure sometimes puts multiple messages or whitespace between them)
			msgStr := strings.TrimSpace(line)
			if !strings.HasPrefix(msgStr, "<") {
				// Find the start of the message
				start := strings.Index(msgStr, "<")
				if start == -1 {
					continue
				}
				msgStr = msgStr[start:]
			}

			// Process the received TPCI message
			event := ParseTPCIResponse(msgStr)
			if event != nil {
				select {
				case c.events <- event:
				case <-ctx.Done():
					return
				case <-c.done:
					return
				}
			}
		}
	}
}

// parseShureResponse is no longer used, replaced by ParseTPCIResponse
func (c *shureController) parseShureResponse(response string) interface{} {
	return ParseTPCIResponse(response)
}
