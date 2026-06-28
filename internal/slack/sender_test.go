package slack

import "testing"

func TestNewSenderNotNil(t *testing.T) {
	if NewSender("xoxb-test") == nil {
		t.Fatal("NewSender returned nil")
	}
}
