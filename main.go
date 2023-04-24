package main

import (
	"github/BeanCodeDe/SpaceLight-Planet/server"
	"time"
)

func main() {
	server, err := server.NewServer(":1203")
	if err != nil {
		panic(err)
	}
	server.StartServer()
	time.Sleep(1 * time.Minute)
	defer server.CloseServer()
}
