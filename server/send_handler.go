package server

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/google/uuid"
)

func (server *PlanetServer) SendToPlayer(playerId uuid.UUID, dataMessage *DataMessage) error {
	player := server.players[playerId]
	if player == nil {
		return fmt.Errorf("player [%v] not found", playerId)
	}
	data, err := json.Marshal(dataMessage)
	if err != nil {
		return fmt.Errorf("error while parsing data to send: %v", err)
	}
	if dataMessage.NeedAck {
		server.addWaitForAckMessage(playerId, dataMessage)
	}
	return server.sendMessage(player.addr, data)
}

func (server *PlanetServer) SendToEveryone(dataMessage *DataMessage) error {
	data, err := json.Marshal(dataMessage)
	if err != nil {
		return fmt.Errorf("error while parsing data to send: %v", err)
	}

	for playerId, player := range server.players {
		if dataMessage.NeedAck {
			server.addWaitForAckMessage(playerId, dataMessage)
		}
		if err := server.sendMessage(player.addr, data); err != nil {
			return err
		}
	}
	return nil
}

func (server *PlanetServer) sendMessage(addr net.Addr, data []byte) error {
	_, err := server.udpServer.WriteTo(data, addr)
	if err != nil {
		return fmt.Errorf("error while sending data to send: %v", err)
	}
	return nil
}
