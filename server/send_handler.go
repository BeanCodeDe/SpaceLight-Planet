package server

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/google/uuid"
)

func (server *UdpServer) SendToClient(clientId uuid.UUID, dataMessage *DataMessage) error {
	client := server.clients[clientId]
	if client == nil {
		return fmt.Errorf("client [%v] not found", clientId)
	}
	data, err := json.Marshal(dataMessage)
	if err != nil {
		return fmt.Errorf("error while parsing data to send: %v", err)
	}
	if dataMessage.NeedAck {
		server.addWaitForAckMessage(clientId, dataMessage)
	}
	return server.sendMessage(client.addr, data)
}

func (server *UdpServer) SendToEveryone(dataMessage *DataMessage) error {
	data, err := json.Marshal(dataMessage)
	if err != nil {
		return fmt.Errorf("error while parsing data to send: %v", err)
	}

	for clientId, client := range server.clients {
		if dataMessage.NeedAck {
			server.addWaitForAckMessage(clientId, dataMessage)
		}
		if err := server.sendMessage(client.addr, data); err != nil {
			return err
		}
	}
	return nil
}

func (server *UdpServer) sendMessage(addr net.Addr, data []byte) error {
	_, err := server.udpServer.WriteTo(data, addr)
	if err != nil {
		return fmt.Errorf("error while sending data to send: %v", err)
	}
	return nil
}
