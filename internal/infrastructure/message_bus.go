package infrastructure

import (
	"context"
	"sync"
)

// MessageBus defines the interface for internal message passing
type MessageBus interface {
	Send(msg Message) error
	Receive() <-chan Message
}

// Message represents a generic message passed between components
type Message struct {
	Type    MessageType
	Payload interface{}
	Source  string // Origin address (e.g., Shure IP)
}

// MessageType defines the type of message
type MessageType string

const (
	ShureDeviceMsg MessageType = "shure_device"
	NMOSNodeMsg    MessageType = "nmos_node"
)

// InMemoryMessageBus is a simple in-memory implementation of MessageBus
type InMemoryMessageBus struct {
	sendChan    chan Message
	receiveChan chan Message
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewInMemoryMessageBus creates a new in-memory message bus
func NewInMemoryMessageBus() MessageBus {
	ctx, cancel := context.WithCancel(context.Background())
	mb := &InMemoryMessageBus{
		sendChan:    make(chan Message, 100),
		receiveChan: make(chan Message, 100),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start goroutine to handle message passing
	mb.wg.Add(1)
	go func() {
		defer mb.wg.Done()
		for {
			select {
			case <-mb.ctx.Done():
				return
			case msg := <-mb.sendChan:
				mb.receiveChan <- msg
			}
		}
	}()

	return mb
}

// Send sends a message to the bus
func (mb *InMemoryMessageBus) Send(msg Message) error {
	select {
	case mb.sendChan <- msg:
		return nil
	case <-mb.ctx.Done():
		return context.Canceled
	}
}

// Receive returns a channel for receiving messages
func (mb *InMemoryMessageBus) Receive() <-chan Message {
	return mb.receiveChan
}

// Close closes the message bus
func (mb *InMemoryMessageBus) Close() {
	mb.cancel()
	mb.wg.Wait()
	close(mb.sendChan)
	close(mb.receiveChan)
}
