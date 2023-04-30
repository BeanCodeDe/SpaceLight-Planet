package server

import (
	"encoding/json"
	"fmt"
	"time"
)

func (server *UdpServer) readIncomingRequests(closedChan chan bool) {
	server.logger.Debugf("Start read incoming requests job.")
	for !isClosed(closedChan, 1*time.Nanosecond) {
		buf := make([]byte, 1024)
		readSize, addr, err := server.udpServer.ReadFrom(buf)
		if err != nil {
			server.logger.Warnf("Error while reading incoming data: %v", err)
			continue
		}
		trimedBuffer := buf[:readSize]
		server.receiveMessageChan <- &receivedMessage{time: time.Now(), data: trimedBuffer, sender: addr}
	}
	server.logger.Debugf("Stopped read incoming requests job.")
}

func (server *UdpServer) processIncomingRequests(closedChan chan bool) {
	server.logger.Debugf("Start process incoming requests job.")
	for !isClosed(closedChan, 1*time.Nanosecond) {
		receivedMessage := <-server.receiveMessageChan
		dataMessage := &DataMessage{}
		if err := json.Unmarshal(receivedMessage.data, dataMessage); err != nil {
			server.logger.Warnf("Error while parsing incoming data: %v", err)
			continue
		}

		if _, exists := server.receivedMessageIds[dataMessage.Id]; exists {
			server.logger.Infof("Message already recived: %v", dataMessage.Id)
			continue
		}

		if err := server.validateLag(dataMessage.SendTime, receivedMessage.time); err != nil {
			server.logger.Warnf("Error while validating lag: %v", err)
			continue
		}

		server.receivedMessageIds[dataMessage.Id] = time.Now()
		if _, exists := server.clients[dataMessage.ClientId]; !exists {
			server.clientConnectHandler <- dataMessage.ClientId
		}
		server.clients[dataMessage.ClientId] = &client{addr: receivedMessage.sender, lastReceivedMessage: time.Now()}

		if dataMessage.NeedAck {
			server.ackData.ackHandlerChan <- dataMessage
		}

		handler, exists := server.messageHandler[dataMessage.Topic]
		if !exists {
			server.logger.Warnf("no handler for topic %s was found", dataMessage.Topic)
			continue
		}

		handler <- dataMessage
	}
	server.logger.Debugf("Stopped process incoming requests job.")
}

func (server *UdpServer) validateLag(sendTime time.Time, receiveTime time.Time) error {
	durationProcess := receiveTime.Sub(sendTime)
	if durationProcess.Milliseconds() > lag_process_time {
		return fmt.Errorf("server is lagging: %vms", durationProcess.Microseconds())
	}
	currentTime := time.Now()
	durationReceive := currentTime.Sub(sendTime)
	if durationReceive.Milliseconds() > lag_receive_time {
		return fmt.Errorf("client is lagging: %vms", durationReceive.Microseconds())
	}
	return nil
}

func (server *UdpServer) checkInactiveClient(closedChan chan bool) {
	server.logger.Debugf("Start check inactive client job.")
	for !isClosed(closedChan, 250*time.Millisecond) {
		currentTime := time.Now()
		for index, client := range server.clients {
			lastMessageDuration := currentTime.Sub(client.lastReceivedMessage)
			if lastMessageDuration.Milliseconds() > leave_client {
				server.clientLeaveHandler <- index
				server.logger.Infof("Client %v didn't send message since %dms and will be deleted", index, lastMessageDuration.Milliseconds())
				delete(server.clients, index)
			} else if lastMessageDuration.Milliseconds() > lag_client {
				server.clientLagHandler <- index
				server.logger.Infof("Client %v didn't send message since %dms and is lagging", index, lastMessageDuration.Milliseconds())
			}
		}
	}
	server.logger.Debugf("Stopped check inactive client job.")
}
