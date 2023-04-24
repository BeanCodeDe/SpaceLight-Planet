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
	leave_client     = 5000
	lag_client       = 1000
)

type (
	Handler interface {
		HandleMessage(*DataMessage)
	}

	ClientEventHandler interface {
		HandleEvent(uuid.UUID)
	}

	Server interface {
		SetLogger(logger Logger)
		StartServer()
		CloseServer()
		AddHandler(topic string, handler Handler)
		SetClientConnectHandler(handler ClientEventHandler)
		SetClientLagHandler(handler ClientEventHandler)
		SetClientLeaveHandler(handler ClientEventHandler)
		SendToEveryone(dataMessage *DataMessage) error
		SendToClient(clientId uuid.UUID, dataMessage *DataMessage) error
	}

	UdpServer struct {
		isClosed             bool
		udpServer            net.PacketConn
		clientConnectHandler chan uuid.UUID
		clientLagHandler     chan uuid.UUID
		clientLeaveHandler   chan uuid.UUID
		messageHandler       map[string]chan *DataMessage
		clients              map[uuid.UUID]*client

		receiveMessageChan chan *receivedMessage
		receivedMessageIds map[uuid.UUID]time.Time

		ackData *ackData

		logger Logger
	}

	sendMessage struct {
		sendToClient uuid.UUID
		lastRetry    time.Time
		retries      int
		dataMessage  *DataMessage
	}

	client struct {
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
		ClientId uuid.UUID              `json:"client_id"`
		Topic    string                 `json:"topic"`
		Data     map[string]interface{} `json:"data"`
	}
)

func NewServer(addr string) (Server, error) {
	udpServer, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("an error aaccourd while creating udp server: %v", err)
	}
	server := &UdpServer{
		isClosed:             false,
		udpServer:            udpServer,
		clientConnectHandler: make(chan uuid.UUID),
		clientLagHandler:     make(chan uuid.UUID),
		clientLeaveHandler:   make(chan uuid.UUID),
		messageHandler:       make(map[string]chan *DataMessage),
		clients:              make(map[uuid.UUID]*client),

		receiveMessageChan: make(chan *receivedMessage),
		receivedMessageIds: make(map[uuid.UUID]time.Time),

		ackData: newAckData(),

		logger: &nilLogger{},
	}
	return server, nil
}

func (server *UdpServer) SetLogger(logger Logger) {
	logger.Debugf("Set logger")
	server.logger = logger
}

func (server *UdpServer) StartServer() {
	server.logger.Debugf("Start server")
	go server.startCleanUpCache()
	go server.startAckHandle()
	go server.startAckWatcher()
	go server.checkInactiveClient()
	go server.processIncomingRequests()
	go server.readIncomingRequests()
}

func (server *UdpServer) CloseServer() {
	server.logger.Debugf("Close server")
	server.udpServer.Close()
}

func (server *UdpServer) AddHandler(topic string, handler Handler) {
	server.logger.Debugf("Add handler for topic %s", topic)
	dataMessageChan := make(chan *DataMessage)
	server.messageHandler[topic] = dataMessageChan
	server.startHandler(dataMessageChan, handler)

}

func (server *UdpServer) startHandler(dataMessageChan chan *DataMessage, handler Handler) {
	server.logger.Debugf("Start handler")
	go func() {
		for !server.isClosed {
			data := <-dataMessageChan
			handler.HandleMessage(data)
		}
	}()
}

func (server *UdpServer) SetClientConnectHandler(handler ClientEventHandler) {
	server.logger.Debugf("Set client connect handler")
	server.startClientEventHandler(server.clientConnectHandler, handler)
}
func (server *UdpServer) SetClientLagHandler(handler ClientEventHandler) {
	server.logger.Debugf("Set client lag handler")
	server.startClientEventHandler(server.clientLagHandler, handler)
}
func (server *UdpServer) SetClientLeaveHandler(handler ClientEventHandler) {
	server.logger.Debugf("Set client leave handler")
	server.startClientEventHandler(server.clientLeaveHandler, handler)
}

func (server *UdpServer) startClientEventHandler(clientIdChan chan uuid.UUID, handler ClientEventHandler) {
	server.logger.Debugf("Start client event handler")
	go func() {
		for !server.isClosed {
			clientId := <-clientIdChan
			handler.HandleEvent(clientId)
		}
	}()
}

func (server *UdpServer) startCleanUpCache() {
	server.logger.Debugf("Start clean up cache job")
	for !server.isClosed {
		time.Sleep(1 * time.Minute)
		toDeleteList := make([]uuid.UUID, 0)
		now := time.Now()
		for index, time := range server.receivedMessageIds {
			if now.Sub(time).Seconds() > leave_client {
				toDeleteList = append(toDeleteList, index)
			}
		}
		server.logger.Debugf("Delete %d old messages from cache.", len(toDeleteList))
		for _, toDelete := range toDeleteList {
			delete(server.receivedMessageIds, toDelete)
		}
	}
}
