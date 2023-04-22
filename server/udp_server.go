package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

const (
	lag_process_time = 200
	lag_receive_time = 1000
	lag_player       = 5000
)

type (
	Handler interface {
		HandleMessage(*DataMessage)
	}

	Server interface {
		CloseServer()
		AddHandler(topic string, handler Handler)
		SendToEveryone(dataMessage *DataMessage) error
		SendToPlayer(playerId uuid.UUID, dataMessage *DataMessage) error
	}

	PlanetServer struct {
		isClosed           bool
		udpServer          net.PacketConn
		receiveMessageChan chan *receivedMessage
		ackHandler         chan *DataMessage
		messageHandler     map[string]chan *DataMessage
		players            map[uuid.UUID]*player
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
		isClosed:           false,
		udpServer:          udpServer,
		receiveMessageChan: make(chan *receivedMessage),
		messageHandler:     make(map[string]chan *DataMessage),
	}
	go server.checkInactivePlayer()
	go server.readIncomingRequests()
	go server.processIncomingRequests()
	return server, nil
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

func (server *PlanetServer) checkInactivePlayer() {
	for !server.isClosed {
		currentTime := time.Now()
		for index, player := range server.players {
			lastMessageDuration := currentTime.Sub(player.lastReceivedMessage)
			if lastMessageDuration.Milliseconds() > lag_player {
				//TODO player lagging handler
				fmt.Printf("Player %v didn't send message since %dms and will be deleted", index, lastMessageDuration.Milliseconds())
				server.players[index] = nil
			}
		}
	}
}

func (server *PlanetServer) readIncomingRequests() {
	for !server.isClosed {
		buf := make([]byte, 1024)
		_, addr, err := server.udpServer.ReadFrom(buf)
		if err != nil {
			fmt.Println(err)
			continue
		}
		server.receiveMessageChan <- &receivedMessage{time: time.Now(), data: buf, sender: addr}
	}
}

func (server *PlanetServer) processIncomingRequests() {
	for !server.isClosed {
		receivedMessage := <-server.receiveMessageChan
		dataMessage := &DataMessage{}
		if err := json.Unmarshal(receivedMessage.data, dataMessage); err != nil {
			fmt.Println(err)
			continue
		}

		if err := server.validateLag(dataMessage.SendTime, receivedMessage.time); err != nil {
			fmt.Printf("Error while validating lag: %v", err)
			continue
		}

		if err := server.validateToken(dataMessage.PlayerId, dataMessage.Token); err != nil {
			fmt.Printf("Error while validating token: %v", err)
			continue
		}
		server.players[dataMessage.PlayerId] = &player{addr: receivedMessage.sender, lastReceivedMessage: time.Now()}

		if dataMessage.NeedAck {
			server.ackHandler <- dataMessage
		}

		handler := server.messageHandler[dataMessage.Topic]
		if handler == nil {
			fmt.Printf("no handler for topic %s was found", dataMessage.Topic)
			continue
		}

		handler <- dataMessage
	}

}

func (server *PlanetServer) validateLag(sendTime time.Time, receiveTime time.Time) error {
	durationProcess := receiveTime.Sub(sendTime)
	if durationProcess.Milliseconds() > lag_process_time {
		return fmt.Errorf("server is lagging: %v", durationProcess.Microseconds())
	}
	currentTime := time.Now()
	durationReceive := currentTime.Sub(sendTime)
	if durationReceive.Milliseconds() > lag_receive_time {
		return fmt.Errorf("client is lagging: %v", durationReceive.Microseconds())
	}
	return nil
}

func (server *PlanetServer) validateToken(playerId uuid.UUID, token string) error {
	return nil
}

func (server *PlanetServer) SendToPlayer(playerId uuid.UUID, dataMessage *DataMessage) error {
	player := server.players[playerId]
	if player == nil {
		return fmt.Errorf("player [%v] not found", playerId)
	}
	data, err := json.Marshal(dataMessage)
	if err != nil {
		return fmt.Errorf("error while parsing data to send: %v", err)
	}
	return server.sendMessage(player.addr, data)
}

func (server *PlanetServer) SendToEveryone(dataMessage *DataMessage) error {
	data, err := json.Marshal(dataMessage)
	if err != nil {
		return fmt.Errorf("error while parsing data to send: %v", err)
	}
	for _, player := range server.players {
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
