package server

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

const (
	lag_process_time              = 200
	lag_receive_time              = 1000
	leave_client                  = 3000
	lag_client                    = 500
	client_connect_handler_buffer = 10
	client_lag_handler_buffer     = 10
	client_leave_handler_buffer   = 10
	message_handler_buffer        = 10
	receive_message_buffer        = 10
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
		closeChanList        []chan bool
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
		closeChanList:        make([]chan bool, 0),
		udpServer:            udpServer,
		clientConnectHandler: make(chan uuid.UUID, client_connect_handler_buffer),
		clientLagHandler:     make(chan uuid.UUID, client_lag_handler_buffer),
		clientLeaveHandler:   make(chan uuid.UUID, client_leave_handler_buffer),
		messageHandler:       make(map[string]chan *DataMessage),
		clients:              make(map[uuid.UUID]*client),

		receiveMessageChan: make(chan *receivedMessage, receive_message_buffer),
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
	closedCleanUpCacheChan := server.getCloseSignal()
	go server.startCleanUpCache(closedCleanUpCacheChan)
	closedAckHandleChan := server.getCloseSignal()
	go server.startAckHandle(closedAckHandleChan)
	closedAckWatcherChan := server.getCloseSignal()
	go server.startAckWatcher(closedAckWatcherChan)
	closedInactiveClientChan := server.getCloseSignal()
	go server.checkInactiveClient(closedInactiveClientChan)
	closedprocessIncomingRequestsChan := server.getCloseSignal()
	go server.processIncomingRequests(closedprocessIncomingRequestsChan)
	closedreadIncomingRequestsChan := server.getCloseSignal()
	go server.readIncomingRequests(closedreadIncomingRequestsChan)
}

func (server *UdpServer) CloseServer() {
	server.logger.Debugf("Close server")
	for _, closeChan := range server.closeChanList {
		closeChan <- true
	}
	server.udpServer.Close()
}

func (server *UdpServer) AddHandler(topic string, handler Handler) {
	server.logger.Debugf("Add handler for topic %s", topic)
	dataMessageChan := make(chan *DataMessage, message_handler_buffer)
	server.messageHandler[topic] = dataMessageChan
	server.startHandler(dataMessageChan, handler)

}

func (server *UdpServer) startHandler(dataMessageChan chan *DataMessage, handler Handler) {
	server.logger.Debugf("Start handler")
	closedChan := server.getCloseSignal()
	go func() {
		for !isClosed(closedChan, 1*time.Nanosecond) {
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
	closedChan := server.getCloseSignal()
	go func() {
		for !isClosed(closedChan, 1*time.Nanosecond) {
			clientId := <-clientIdChan
			handler.HandleEvent(clientId)
		}
	}()
}

func (server *UdpServer) getCloseSignal() chan bool {
	newCloseSignal := make(chan bool)
	server.closeChanList = append(server.closeChanList, newCloseSignal)
	return newCloseSignal
}

func isClosed(closeSignal chan bool, waitTime time.Duration) bool {
	select {
	case closed := <-closeSignal:
		return closed
	case <-time.After(waitTime):
		return false
	}
}

func (server *UdpServer) startCleanUpCache(closedChan chan bool) {
	server.logger.Debugf("Start clean up cache job")

	for !isClosed(closedChan, 1*time.Minute) {
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
