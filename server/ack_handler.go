package server

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	topic_ack     = "ACK"
	ack_timeout   = 200
	ack_max_retry = 3
)

type (
	ackHandler struct {
		server *UdpServer
	}

	ackData struct {
		ackHandlerChan    chan *DataMessage
		waitForAckMessage []*sendMessage
		waitForAckMutex   sync.Mutex
	}
)

func newAckData() *ackData {
	return &ackData{
		ackHandlerChan:    make(chan *DataMessage),
		waitForAckMessage: make([]*sendMessage, 0),
	}
}

func (server *UdpServer) startAckHandle() {
	server.logger.Debugf("Start ack handler job.")
	for !server.isClosed {
		data := <-server.ackData.ackHandlerChan
		server.ackHandle(data)
	}
	server.logger.Debugf("Stopped ack handler job.")
}

func (server *UdpServer) ackHandle(dataMessage *DataMessage) {
	ackMessage := &DataMessage{
		Id:       dataMessage.Id,
		SendTime: time.Now(),
		NeedAck:  false,
		ClientId: dataMessage.ClientId,
		Topic:    topic_ack,
		Data:     nil,
	}
	server.SendToClient(dataMessage.ClientId, ackMessage)
}

func (server *UdpServer) startAckWatcher() {
	server.logger.Debugf("Start ack watcher job.")
	server.AddHandler(topic_ack, &ackHandler{server: server})
	for !server.isClosed {
		server.watchAck()
	}
	server.logger.Debugf("Stopped ack watcher job.")
}

func (server *UdpServer) watchAck() {
	server.ackData.waitForAckMutex.Lock()
	defer server.ackData.waitForAckMutex.Unlock()
	indicesToDelete := make([]int, 0)
	for index, ackMessage := range server.ackData.waitForAckMessage {
		now := time.Now()
		if now.Sub(ackMessage.lastRetry).Microseconds() > ack_timeout {
			clientId := ackMessage.sendToClient
			if ackMessage.retries == ack_max_retry {
				server.clientLeaveHandler <- clientId
				indicesToDelete = append(indicesToDelete, index)
				continue
			}
			server.clientLagHandler <- clientId
			client := server.clients[ackMessage.sendToClient]
			if client == nil {
				server.logger.Warnf("client [%v] not found", ackMessage.sendToClient)
				indicesToDelete = append(indicesToDelete, index)
				continue
			}
			data, err := json.Marshal(ackMessage.dataMessage)
			if err != nil {
				server.logger.Warnf("error while parsing data to send: %v\n", err)
			}
			server.sendMessage(client.addr, data)
			ackMessage.lastRetry = time.Now()
			ackMessage.retries = ackMessage.retries + 1
		}
	}
	server.ackData.waitForAckMessage = removeManyElementsByIndices(server.ackData.waitForAckMessage, indicesToDelete)
}

func (handler *ackHandler) HandleMessage(dataMessage *DataMessage) {
	handler.server.ackData.waitForAckMutex.Lock()
	defer handler.server.ackData.waitForAckMutex.Unlock()
	for index, ackMessage := range handler.server.ackData.waitForAckMessage {
		if dataMessage.Id == ackMessage.dataMessage.Id {
			handler.server.ackData.waitForAckMessage = removeElement(handler.server.ackData.waitForAckMessage, index)
			return
		}
	}
}

func (server *UdpServer) addWaitForAckMessage(clientId uuid.UUID, dataMessage *DataMessage) {
	server.ackData.waitForAckMutex.Lock()
	defer server.ackData.waitForAckMutex.Unlock()
	message := &sendMessage{
		sendToClient: clientId,
		dataMessage:  dataMessage,
		lastRetry:    dataMessage.SendTime,
		retries:      0,
	}
	server.ackData.waitForAckMessage = append(server.ackData.waitForAckMessage, message)
}
