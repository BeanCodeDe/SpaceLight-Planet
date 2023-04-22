package main

import "time"

const topic_ack = "ACK"

type (
	ackHandler struct {
		server Server
	}
)

func NewAckHandler(server Server) Handler {
	return &ackHandler{server: server}
}

func (handler ackHandler) HandleMessage(dataMessage *DataMessage) {
	ackMessage := &DataMessage{
		Id:       dataMessage.Id,
		SendTime: time.Now(),
		NeedAck:  false,
		PlayerId: dataMessage.PlayerId,
		Token:    dataMessage.Token,
		Topic:    topic_ack,
		Data:     nil,
	}
	handler.server.SendToPlayer(dataMessage.PlayerId, ackMessage)
}
