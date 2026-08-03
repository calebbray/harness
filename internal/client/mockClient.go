package client

type MockClient struct {
	inCh    chan string
	outCh   chan Content
	history []Message
}

func NewMockClient() *MockClient {
	return &MockClient{
		inCh:    make(chan string),
		outCh:   make(chan Content),
		history: make([]Message, 0),
	}
}

func (c *MockClient) waitForMessage() {
	for {
		query := <-c.inCh
		c.handleQuery(query)
	}
}

func (c *MockClient) Consume() <-chan Content {
	return c.outCh
}

func (c *MockClient) SendMessage(msg string) error {
	c.inCh <- msg

	return nil
}

func (c *MockClient) handleQuery(question string) error {
	c.outCh <- TextContent{ContentType: "text", Text: question + " " + "pong"}
	return nil
}
