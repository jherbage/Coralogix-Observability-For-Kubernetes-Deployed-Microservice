package message

import (
	"encoding/json"
	"fmt"
)

// Message represents the schema for a message
type Message struct {
	ID       string `json:"id"`        // Unique identifier for the message
	Content  string `json:"content"`   // The content of the message
	WaitTime int    `json:"wait_time"` // Wait time in seconds
}

// NewMessage creates a new Message instance
func NewMessage(id, content string, waitTime int) *Message {
	return &Message{
		ID:       id,
		Content:  content,
		WaitTime: waitTime,
	}
}

// ToJSON converts the Message to a JSON string
func (m *Message) ToJSON() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}
	return string(data), nil
}

// FromJSON parses a JSON string into a Message
func FromJSON(data string) (*Message, error) {
	var msg Message
	err := json.Unmarshal([]byte(data), &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}
