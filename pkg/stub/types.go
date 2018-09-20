package stub

import (
	"context"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

// HandlerFunc type to provide a way to use a function as a handler rather than a struct
type HandlerFunc func(context context.Context, event sdk.Event) error

func (h HandlerFunc) Handle(context context.Context, event sdk.Event) error {
	return h(context, event)
}
