package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

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

		if _, exists := server.receivedMessageIds[dataMessage.Id]; exists {
			fmt.Printf("Message already recived: %v", dataMessage.Id)
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
		server.receivedMessageIds[dataMessage.Id] = time.Now()
		if _, exists := server.players[dataMessage.PlayerId]; !exists {
			server.playerConnectHandler <- dataMessage.PlayerId
		}
		server.players[dataMessage.PlayerId] = &player{addr: receivedMessage.sender, lastReceivedMessage: time.Now()}

		if dataMessage.NeedAck {
			server.ackData.ackHandlerChan <- dataMessage
		}

		handler, exists := server.messageHandler[dataMessage.Topic]
		if !exists {
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

func (server *PlanetServer) checkInactivePlayer() {
	for !server.isClosed {
		currentTime := time.Now()
		for index, player := range server.players {
			lastMessageDuration := currentTime.Sub(player.lastReceivedMessage)
			if lastMessageDuration.Milliseconds() > leave_player {
				server.playerLeaveHandler <- index
				fmt.Printf("Player %v didn't send message since %dms and will be deleted", index, lastMessageDuration.Milliseconds())
				delete(server.players, index)
			} else if lastMessageDuration.Milliseconds() > lag_player {
				server.playerLagHandler <- index
				fmt.Printf("Player %v didn't send message since %dms and is lagging", index, lastMessageDuration.Milliseconds())
			}
		}
	}
}
