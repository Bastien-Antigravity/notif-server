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

	"github.com/Bastien-Antigravity/universal-logger/src/utils"

	capnplib "capnproto.org/go/capnp/v3"
)

type NotifNcapHandler struct {
	Name string
}

// -----------------------------------------------------------------------------

func NewNotifHandler(name string) *NotifNcapHandler {
	return &NotifNcapHandler{Name: name}
}

// -----------------------------------------------------------------------------

// NotifNcapSerialize serializes a NotifMessage into packed Cap'n Proto binary format.
// Allocates a fresh message arena per call to avoid unbounded memory leaks and data races [NS-6].
func (notifNcapHandler *NotifNcapHandler) NotifNcapSerialize(notifMessage *utils.NotifMessage) []byte {
	if notifMessage == nil {
		return nil
	}

	capnplibMsg, memSeg, err := capnplib.NewMessage(capnplib.SingleSegment(nil))
	if err != nil {
		return nil
	}

	notifObj, err := notifMsg.NewRootNotifierMsg(memSeg)
	if err != nil {
		return nil
	}

	_ = notifObj.SetMessage_(notifMessage.Message)
	_ = notifObj.SetAttachment(notifMessage.Attachment)
	_ = notifObj.SetLevel(notifMessage.Level)

	// Create new text list for tags
	tList, err := capnplib.NewTextList(memSeg, int32(len(notifMessage.Tags)))
	if err == nil {
		for i, val := range notifMessage.Tags {
			_ = tList.Set(i, val)
		}
		_ = notifObj.SetTags(tList)
	}

	byteMsg, err := capnplibMsg.MarshalPacked()
	if err != nil {
		return nil
	}
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
