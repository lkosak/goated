package gateway

import "context"

type IncomingMessage struct {
	Channel string
	ChatID  string
	UserID  string
	Text    string
}

type Responder interface {
	SendMessage(ctx context.Context, chatID, text string) error
}

type Handler interface {
	HandleMessage(ctx context.Context, msg IncomingMessage, responder Responder) error
}

type Connector interface {
	Run(ctx context.Context, handler Handler) error
}
