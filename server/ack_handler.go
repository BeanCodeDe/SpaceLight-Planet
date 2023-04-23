package server

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	topic_ack     = "ACK"
	ack_timeout   = 100
	ack_max_retry = 3
)

type (
	ackHandler struct {
		server *PlanetServer
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

func (server *PlanetServer) startAckHandle() {
	for !server.isClosed {
		data := <-server.ackData.ackHandlerChan
		server.ackHandle(data)
	}
}

func (server *PlanetServer) ackHandle(dataMessage *DataMessage) {
	ackMessage := &DataMessage{
		Id:       dataMessage.Id,
		SendTime: time.Now(),
		NeedAck:  false,
		PlayerId: dataMessage.PlayerId,
		Token:    dataMessage.Token,
		Topic:    topic_ack,
		Data:     nil,
	}
	server.SendToPlayer(dataMessage.PlayerId, ackMessage)
}

func (server *PlanetServer) startAckWatcher() {
	server.AddHandler(topic_ack, &ackHandler{server: server})
	for !server.isClosed {
		server.watchAck()
	}
}

func (server *PlanetServer) watchAck() {
	server.ackData.waitForAckMutex.Lock()
	defer server.ackData.waitForAckMutex.Unlock()
	indicesToDelete := make([]int, 0)
	for index, ackMessage := range server.ackData.waitForAckMessage {
		now := time.Now()
		if now.Sub(ackMessage.lastRetry).Microseconds() > ack_timeout {
			playerId := ackMessage.sendToPlayer
			if ackMessage.retries == ack_max_retry {
				server.playerLeaveHandler <- playerId
				indicesToDelete = append(indicesToDelete, index)
				continue
			}
			server.playerLagHandler <- playerId
			player := server.players[ackMessage.sendToPlayer]
			if player == nil {
				fmt.Printf("player [%v] not found\n", ackMessage.sendToPlayer)
				indicesToDelete = append(indicesToDelete, index)
				continue
			}
			data, err := json.Marshal(ackMessage.dataMessage)
			if err != nil {
				fmt.Printf("error while parsing data to send: %v\n", err)
			}
			server.sendMessage(player.addr, data)
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

func (server *PlanetServer) addWaitForAckMessage(playerId uuid.UUID, dataMessage *DataMessage) {
	server.ackData.waitForAckMutex.Lock()
	defer server.ackData.waitForAckMutex.Unlock()
	message := &sendMessage{
		sendToPlayer: playerId,
		dataMessage:  dataMessage,
		lastRetry:    dataMessage.SendTime,
		retries:      0,
	}
	server.ackData.waitForAckMessage = append(server.ackData.waitForAckMessage, message)
}
