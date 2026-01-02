package interfaces

type NotifSenderInterface interface {
	SendMessage(msg, to, subject string) error
}
