package tests

import (
	"logs-distributor/config"
	"logs-distributor/distributor/implementations"
	"logs-distributor/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPacketValidator_ValidPacket(t *testing.T) {
	validator := implementations.NewPacketValidator()

	packet := models.LogPacket{
		ID: "test",
		Messages: []models.LogMessage{
			{ID: "msg1", Level: "INFO", Message: "test", Source: "test"},
		},
	}

	err := validator.ValidatePacket(packet)
	assert.NoError(t, err)
}

func TestPacketValidator_EmptyPacket(t *testing.T) {
	validator := implementations.NewPacketValidator()

	packet := models.LogPacket{
		ID:       "test",
		Messages: []models.LogMessage{},
	}

	err := validator.ValidatePacket(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one message")
}

func TestPacketValidator_OversizedMessage(t *testing.T) {
	validator := implementations.NewPacketValidator()

	packet := models.LogPacket{
		ID: "test",
		Messages: []models.LogMessage{
			{ID: "msg1", Level: "INFO", Message: string(make([]byte, config.MaxLogMessageLength+1)), Source: "test"},
		},
	}

	err := validator.ValidatePacket(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message length")
}

func TestPacketValidator_TooManyMessages(t *testing.T) {
	validator := implementations.NewPacketValidator()

	// Create packet with too many messages
	messages := make([]models.LogMessage, config.MaxMessagesPerPacket+1)
	for i := range messages {
		messages[i] = models.LogMessage{
			ID: "msg", Level: "INFO", Message: "test", Source: "test",
		}
	}

	packet := models.LogPacket{
		ID:       "test",
		Messages: messages,
	}

	err := validator.ValidatePacket(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum allowed")
}

func TestPacketValidator_PacketTooLarge(t *testing.T) {
	validator := implementations.NewPacketValidator()

	// Create a message that's just under the individual limit but packet exceeds total
	messageSize := config.MaxLogMessageLength / 2
	numMessages := (config.MaxPacketSizeBytes / messageSize) + 1

	messages := make([]models.LogMessage, numMessages)
	for i := range messages {
		messages[i] = models.LogMessage{
			ID: "msg", Level: "INFO", Message: string(make([]byte, messageSize)), Source: "test",
		}
	}

	packet := models.LogPacket{
		ID:       "test",
		Messages: messages,
	}

	err := validator.ValidatePacket(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "packet size")
}
