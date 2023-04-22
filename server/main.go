package main

func main() {
	server, err := NewServer()
	if err != nil {
		panic(err)
	}
	ackHandler := NewAckHandler(server)
	server.AddHandler(topic_ack, ackHandler)
}
