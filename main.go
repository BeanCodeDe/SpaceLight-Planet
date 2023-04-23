package main

import "github/BeanCodeDe/SpaceLight-Planet/server"

func main() {
	server, err := server.NewServer()
	if err != nil {
		panic(err)
	}
	server.StartServer()
	defer server.CloseServer()
}
