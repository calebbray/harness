package client

import (
	"testing"
	"time"
)

func TestMockClientWaitAndRespond(t *testing.T) {
	c := NewMockClient()

	go c.waitForMessage()

	time.Sleep(time.Millisecond * 500)
	c.SendMessage("ping")
	for msg := range c.Consume() {
		if content, ok := msg.(TextContent); ok {
			t.Log(content.Text)
			return
		}
	}
}
