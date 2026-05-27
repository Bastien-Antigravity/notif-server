package interfaces

import "context"

type NotifSenderInterface interface {
	SendMessage(ctx context.Context, msg, to, subject string) error
}
