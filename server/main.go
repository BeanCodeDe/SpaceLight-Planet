package main

func main() {
	server, err := NewServer()
	if err != nil {
		panic(err)
	}
	server.StartServer()
}
