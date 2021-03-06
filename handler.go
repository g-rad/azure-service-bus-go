package servicebus

import "context"

type (
	// Handler exposes the functionality required to process a Service Bus message.
	Handler interface {
		Handle(context.Context, *Message) DispositionAction
	}

	// HandlerFunc is a type converter that allows a func to be used as a `Handler`
	HandlerFunc func(context.Context, *Message) DispositionAction

	// SessionHandler exposes a manner of handling a group of messages together. Instances of SessionHandler should be
	// passed to a Receiver such as a Queue or Subscription.
	SessionHandler interface {
		Handler

		// Start is called when a Receiver is informed that has acquired a lock on a Service Bus Session.
		Start(*MessageSession) error

		// End is called when a Receiver is informed that the last message of a Session has been passed to it.
		End()
	}
)

// Handle redirects this call to the func that was provided.
func (hf HandlerFunc) Handle(ctx context.Context, msg *Message) DispositionAction {
	return hf(ctx, msg)
}

type defaultSessionHandler struct {
	Handler
	start func(*MessageSession) error
	end   func()
}

// NewSessionHandler is a type converter that allows three funcs to be tied together into a type that fulfills the
// SessionHandler interface.
func NewSessionHandler(base Handler, start func(*MessageSession) error, end func()) SessionHandler {
	return &defaultSessionHandler{
		Handler: base,
		start:   start,
		end:     end,
	}
}

// Start calls the func() that was provided to `NewSessionHandler` when a new session lock is established.
func (dsh defaultSessionHandler) Start(ms *MessageSession) error {
	return dsh.start(ms)
}

// End calls the func() that was provided to `NewSessionHandler` when a session is finished processing for any reason.
func (dsh defaultSessionHandler) End() {
	dsh.end()
}
