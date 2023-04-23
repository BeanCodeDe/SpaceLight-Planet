package server

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

const (
	lag_process_time = 200
	lag_receive_time = 1000
	leave_player     = 5000
	lag_player       = 1000
)

type (
	Handler interface {
		HandleMessage(*DataMessage)
	}

	PlayerEventHandler interface {
		HandleEvent(uuid.UUID)
	}

	Server interface {
		StartServer()
		CloseServer()
		AddHandler(topic string, handler Handler)
		SetPlayerConnectHandler(handler PlayerEventHandler)
		SetPlayerLagHandler(handler PlayerEventHandler)
		SetPlayerLeaveHandler(handler PlayerEventHandler)
		SendToEveryone(dataMessage *DataMessage) error
		SendToPlayer(playerId uuid.UUID, dataMessage *DataMessage) error
	}

	PlanetServer struct {
		isClosed             bool
		udpServer            net.PacketConn
		playerConnectHandler chan uuid.UUID
		playerLagHandler     chan uuid.UUID
		playerLeaveHandler   chan uuid.UUID
		messageHandler       map[string]chan *DataMessage
		players              map[uuid.UUID]*player

		receiveMessageChan chan *receivedMessage
		receivedMessageIds map[uuid.UUID]time.Time

		ackData *ackData
	}

	sendMessage struct {
		sendToPlayer uuid.UUID
		lastRetry    time.Time
		retries      int
		dataMessage  *DataMessage
	}

	player struct {
		lastReceivedMessage time.Time
		addr                net.Addr
	}

	receivedMessage struct {
		time   time.Time
		data   []byte
		sender net.Addr
	}

	DataMessage struct {
		Id       uuid.UUID              `json:"id"`
		SendTime time.Time              `json:"send_time"`
		NeedAck  bool                   `json:"need_ack"`
		PlayerId uuid.UUID              `json:"player_id"`
		Token    string                 `json:"token"`
		Topic    string                 `json:"topic"`
		Data     map[string]interface{} `json:"data"`
	}
)

func NewServer() (Server, error) {
	udpServer, err := net.ListenPacket("udp", ":1053")
	if err != nil {
		return nil, fmt.Errorf("an error aaccourd while creating udp server: %v", err)
	}
	server := &PlanetServer{
		isClosed:             false,
		udpServer:            udpServer,
		playerConnectHandler: make(chan uuid.UUID),
		playerLagHandler:     make(chan uuid.UUID),
		playerLeaveHandler:   make(chan uuid.UUID),
		messageHandler:       make(map[string]chan *DataMessage),
		players:              make(map[uuid.UUID]*player),

		receiveMessageChan: make(chan *receivedMessage),
		receivedMessageIds: make(map[uuid.UUID]time.Time),

		ackData: newAckData(),
	}
	return server, nil
}

func (server *PlanetServer) StartServer() {
	go server.startCleanUpChache()
	go server.startAckHandle()
	go server.startAckWatcher()
	go server.checkInactivePlayer()
	go server.processIncomingRequests()
	go server.readIncomingRequests()
}

func (server *PlanetServer) CloseServer() {
	server.udpServer.Close()
}

func (server *PlanetServer) AddHandler(topic string, handler Handler) {
	dataMessageChan := make(chan *DataMessage)
	server.messageHandler[topic] = dataMessageChan
	server.startHandler(dataMessageChan, handler)

}

func (server *PlanetServer) startHandler(dataMessageChan chan *DataMessage, handler Handler) {
	go func() {
		for !server.isClosed {
			data := <-dataMessageChan
			handler.HandleMessage(data)
		}
	}()
}

func (server *PlanetServer) SetPlayerConnectHandler(handler PlayerEventHandler) {
	server.startPlayerEventHandler(server.playerConnectHandler, handler)
}
func (server *PlanetServer) SetPlayerLagHandler(handler PlayerEventHandler) {
	server.startPlayerEventHandler(server.playerLagHandler, handler)
}
func (server *PlanetServer) SetPlayerLeaveHandler(handler PlayerEventHandler) {
	server.startPlayerEventHandler(server.playerLeaveHandler, handler)
}

func (server *PlanetServer) startPlayerEventHandler(playerIdChan chan uuid.UUID, handler PlayerEventHandler) {
	go func() {
		for !server.isClosed {
			playerId := <-playerIdChan
			handler.HandleEvent(playerId)
		}
	}()
}

func (server *PlanetServer) startCleanUpChache() {
	for !server.isClosed {
		time.Sleep(1 * time.Minute)
		toDeleteList := make([]uuid.UUID, 0)
		now := time.Now()
		for index, time := range server.receivedMessageIds {
			if now.Sub(time).Seconds() > leave_player {
				toDeleteList = append(toDeleteList, index)
			}
		}
		for _, toDelete := range toDeleteList {
			delete(server.receivedMessageIds, toDelete)
		}
	}
}
