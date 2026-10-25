package shared

import "context"

type accessRequestKey struct{}

type AccessRequestView struct {
	Show    bool
	Error   string
	Success bool
}

func WithAccessRequest(ctx context.Context, v AccessRequestView) context.Context {
	return context.WithValue(ctx, accessRequestKey{}, v)
}
func AccessRequestFromContext(ctx context.Context) AccessRequestView {
	v, _ := ctx.Value(accessRequestKey{}).(AccessRequestView)
	return v
}
