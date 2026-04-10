package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	notifie "github.com/Bastien-Antigravity/notif-server/src/core"
	notifie_interfaces "github.com/Bastien-Antigravity/notif-server/src/interfaces"
	factory "github.com/Bastien-Antigravity/safe-socket"
	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	"github.com/Bastien-Antigravity/universal-logger/src/logger"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
	"github.com/stretchr/testify/assert"
)

func TestIdleTimeoutFix(t *testing.T) {
	// 1. Setup config (using 9998 to avoid conflict)
	conf := distributed_config.New("test")
	conf.Capabilities["notif_server"] = map[string]interface{}{"ip": "127.0.0.1", "port": "9998"}

	ml := &mockLogger{}
	ul := logger.NewUniLog(ml)
	// Manual initialization to avoid starting processMessage goroutine which competes for NotifChan
	nt := &notifie.Notifie{
		Name:           "TimeoutTest",
		NotifChan:      make(chan *utils.NotifMessage),
		RawNotifChan:   make(chan []byte),
		TagToSenderMap: make(map[string]notifie_interfaces.NotifSenderInterface),
	}
	go nt.ConsumeRawMessages()

	srv := NewServer(conf, ul, nt)

	// 2. Start server
	done := make(chan error, 1)
	go func() {
		done <- srv.Start()
	}()
	time.Sleep(500 * time.Millisecond)

	// 3. Connect and wait
	client, err := factory.Create("tcp-hello", "127.0.0.1:9998", "127.0.0.1", "client", true)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	found := false
	for _, l := range ml.CapturedLogs {
		if strings.Contains(l, "listening on 127.0.0.1:9998") {
			found = true
			break
		}
	}
	assert.True(t, found, "Logger should have recorded the TCP listening address")

	// Handler for serialization
	handler := notifie.NewNotifHandler("test", conf)

	// 4. Test Idle Timeout Refresh
	// We will wait 3s, send a message, then wait another 3s.
	// We use the proper Cap'n Proto format.

	fmt.Printf("Connected. Waiting 3 seconds...\n")
	time.Sleep(3 * time.Second)

	msg1 := &utils.NotifMessage{Message: "Tick", Tags: []string{"TEST"}}
	testData1 := handler.NotifNcapSerialize(msg1)
	err = client.Send(testData1)
	assert.NoError(t, err, "First send should succeed at 3s")

	fmt.Printf("Waiting another 3 seconds (Total 6s)...\n")
	time.Sleep(3 * time.Second)

	msg2 := &utils.NotifMessage{Message: "Tock", Tags: []string{"TEST"}}
	testData2 := handler.NotifNcapSerialize(msg2)
	err = client.Send(testData2)
	assert.NoError(t, err, "Second send should succeed at 6s")

	// 5. Verify server received both
	for i := 0; i < 2; i++ {
		select {
		case msg := <-nt.NotifChan:
			fmt.Printf("Server received: %s\n", msg.Message)
			if i == 0 {
				assert.Equal(t, "Tick", msg.Message)
			} else {
				assert.Equal(t, "Tock", msg.Message)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Server did not receive all messages")
		}
	}

	srv.Stop()
	<-done
}
