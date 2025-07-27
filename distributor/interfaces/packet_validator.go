package interfaces

import "logs-distributor/models"

// PacketValidator defines the interface for packet validation
type PacketValidator interface {
	ValidatePacket(packet models.LogPacket) error
}
