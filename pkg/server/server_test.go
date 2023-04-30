package server

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSend(t *testing.T) {
	testLogger := NewFmtLogger()

	server, err := NewServer(":1203")
	assert.Nil(t, err)
	assert.NotNil(t, server)

	server.SetLogger(testLogger)

	server.StartServer()
	logs := waitTillNoLogsComingIn(testLogger.logs, 3*time.Second)
	assert.Len(t, logs, 10)
	/*
		clientId := uuid.New()
		err = testSendDataMessage(&DataMessage{
			Id:       uuid.New(),
			SendTime: time.Now(),
			NeedAck:  false,
			ClientId: clientId,
			Topic:    "TEST_TOPIC",
		})
		assert.Nil(t, err)*/

}

func waitTillNoLogsComingIn(logChan chan string, d time.Duration) []string {
	logs := make([]string, 0)

	for {
		select {
		case log := <-logChan:
			logs = append(logs, log)
		case <-time.After(d):
			return logs
		}
	}

}

func testSendDataMessage(dataMessage *DataMessage) error {
	data, err := json.Marshal(dataMessage)
	if err != nil {
		return err
	}
	return testSendData(data)
}

func testSendData(data []byte) error {
	udpServer, err := net.ResolveUDPAddr("udp", ":1203")

	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		return err
	}

	defer conn.Close()

	_, err = conn.Write(data)
	return err
}

func testReceiveData() ([]byte, error) {
	udpServer, err := net.ResolveUDPAddr("udp", ":1203")

	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpServer)
	if err != nil {
		return nil, err
	}

	defer conn.Close()
	received := make([]byte, 1024)
	_, err = conn.Read(received)
	if err != nil {
		return nil, err
	}
	return received, nil
}

type (
	fmtLogger struct {
		logs chan string
	}
)

func NewFmtLogger() *fmtLogger {
	return &fmtLogger{logs: make(chan string, 100)}
}

func (log fmtLogger) Debugf(format string, args ...interface{}) {
	data := fmt.Sprintf(format, args...)
	log.logs <- "Debug: " + data
	fmt.Printf("Debug: %s\n", data)
}
func (log fmtLogger) Infof(format string, args ...interface{}) {
	data := fmt.Sprintf(format, args...)
	log.logs <- "Info: " + data
	fmt.Printf("Info: %s\n", data)
}
func (log fmtLogger) Warnf(format string, args ...interface{}) {
	data := fmt.Sprintf(format, args...)
	log.logs <- "Warn: " + data
	fmt.Printf("Warn: %s\n", data)
}
func (log fmtLogger) Errorf(format string, args ...interface{}) {
	data := fmt.Sprintf(format, args...)
	log.logs <- "Error: " + data
	fmt.Printf("Error: %s\n", data)
}
