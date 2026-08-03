package provider

import (
	"context"
	"strings"
)

type contextKey int

const (
	key contextKey = iota
)

// AddInContext Adds the provider name in the context.
func AddInContext(ctx context.Context, elementName string) context.Context {
	// 如果没有@，则不做处理
	parts := strings.Split(elementName, "@")
	if len(parts) == 1 {
		return ctx
	}
	// 如果已经有@，则不做处理
	if name, ok := ctx.Value(key).(string); ok && name == parts[1] {
		return ctx
	}
	// 如果没有@，则添加@providerName
	return context.WithValue(ctx, key, parts[1])
}

// GetQualifiedName Gets the fully qualified name.
// 如果没有@，则添加@providerName
func GetQualifiedName(ctx context.Context, elementName string) string {
	parts := strings.Split(elementName, "@")
	if len(parts) == 1 {
		if providerName, ok := ctx.Value(key).(string); ok {
			return MakeQualifiedName(providerName, parts[0])
		}
	}
	// 如果已经有 @，则不做处理
	return elementName
}

// MakeQualifiedName Creates a qualified name for an element.
func MakeQualifiedName(providerName, elementName string) string {
	return elementName + "@" + providerName
}
