package notifier

/*
ESSENTIAL PROCESS:
Handles serialization and deserialization of notification messages using Cap'n Proto.
Provides a high-performance binary encoding for cross-process communication.

DATA FLOW:
1. Sources call NotifNcapSerialize to convert Go structs to binary.
2. Sink calls DeserializeNotifMsg to reconstruct Go structs from binary data.

KEY PARAMETERS:
- notifMessage: The underlying Cap'n Proto message object.
- memSeg: Memory segment used for allocation during serialization.
*/

import (
	"fmt"

	notifMsg "github.com/Bastien-Antigravity/notif-server/src/schemas/capnp"

	distributed_config "github.com/Bastien-Antigravity/distributed-config"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"

	capnplib "capnproto.org/go/capnp/v3"
)

type NotifNcapHandler struct {
	Name         string
	config       *distributed_config.Config
	notifMessage *notifMsg.NotifierMsg
	memSeg       *capnplib.Segment
	msgSerDeSer  *capnplib.Message
}

// -----------------------------------------------------------------------------

func NewNotifHandler(name string, parentClassConfig *distributed_config.Config) *NotifNcapHandler {
	capnplibMsg, memSeg, err := capnplib.NewMessage(capnplib.SingleSegment(nil))
	if err != nil {
		panic(fmt.Sprintf("Error while trying to initialize Notif Handler :'%v'\n", err))
	}

	notifObj, err := notifMsg.NewRootNotifierMsg(memSeg)
	if err != nil {
		panic(fmt.Sprintf("Error while trying to initialize Notif Handler :'%v'\n", err))
	}
	return &NotifNcapHandler{Name: name, config: parentClassConfig, memSeg: memSeg, notifMessage: &notifObj, msgSerDeSer: capnplibMsg}
}

// -----------------------------------------------------------------------------

func (notifNcapHandler *NotifNcapHandler) NotifNcapSerialize(notifMessage *utils.NotifMessage) []byte {
	notifNcapHandler.notifMessage.SetMessage_(notifMessage.Message)
	notifNcapHandler.notifMessage.SetAttachment(notifMessage.Attachment)
	notifNcapHandler.notifMessage.SetLevel(notifMessage.Level)

	// Create new text list for tags
	tList, _ := capnplib.NewTextList(notifNcapHandler.memSeg, int32(len(notifMessage.Tags)))
	for i, val := range notifMessage.Tags {
		tList.Set(i, val)
	}
	notifNcapHandler.notifMessage.SetTags(tList)

	byteMsg, _ := notifNcapHandler.msgSerDeSer.MarshalPacked()
	return byteMsg
}

// -----------------------------------------------------------------------------

// DeserializeNotifMsg parses a raw byte slice (Cap'n Proto packed) into a NotifMessage.
// This helper is exposed for servers or other components using this library.
func DeserializeNotifMsg(data []byte) (*utils.NotifMessage, error) {
	capnpMessage, err := capnplib.UnmarshalPacked(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal packed message: %v", err)
	}
	goObj, err := notifMsg.ReadRootNotifierMsg(capnpMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to read root message: %v", err)
	}

	notifMessage := &utils.NotifMessage{
		Tags: make([]string, 0),
	}

	val, err := goObj.Message_()
	if err == nil {
		notifMessage.Message = val
	}

	val, err = goObj.Attachment()
	if err == nil {
		notifMessage.Attachment = val
	}

	val, err = goObj.Level()
	if err == nil {
		notifMessage.Level = val
	}

	tagList, err := goObj.Tags()
	if err == nil {
		notifMessage.Tags = make([]string, tagList.Len())
		for i := 0; i < tagList.Len(); i++ {
			val, _ := tagList.At(i)
			notifMessage.Tags[i] = val
		}
	}

	return notifMessage, nil
}

// -----------------------------------------------------------------------------

func (notifNcapHandler *NotifNcapHandler) NotifNcapDeSerialize(data []byte) *utils.NotifMessage {
	msg, _ := DeserializeNotifMsg(data)
	return msg
}
