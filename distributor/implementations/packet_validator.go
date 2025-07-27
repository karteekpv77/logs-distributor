package implementations

import (
	"fmt"
	"logs-distributor/config"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
)

// PacketValidator implements the PacketValidator interface
type PacketValidator struct{}

// Ensure PacketValidator implements PacketValidator interface
var _ interfaces.PacketValidator = (*PacketValidator)(nil)

func NewPacketValidator() interfaces.PacketValidator {
	return &PacketValidator{}
}

// ValidatePacket validates incoming log packets
func (v *PacketValidator) ValidatePacket(packet models.LogPacket) error {
	if len(packet.Messages) == 0 {
		return fmt.Errorf("packet must contain at least one message")
	}
	if len(packet.Messages) > config.MaxMessagesPerPacket {
		return fmt.Errorf("packet contains %d messages, maximum allowed is %d", len(packet.Messages), config.MaxMessagesPerPacket)
	}

	totalSize := 0
	for _, msg := range packet.Messages {
		if len(msg.Message) > config.MaxLogMessageLength {
			return fmt.Errorf("message length %d exceeds maximum %d", len(msg.Message), config.MaxLogMessageLength)
		}
		totalSize += len(msg.Message)
	}

	if totalSize > config.MaxPacketSizeBytes {
		return fmt.Errorf("packet size %d bytes exceeds maximum %d bytes", totalSize, config.MaxPacketSizeBytes)
	}

	return nil
}
