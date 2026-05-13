//go:build js && wasm

package bundlewasm

import "syscall/js"

// TaggedError builds a JS Error object with `kind` set to the supplied
// taxonomy value, so JS callers can switch on err.kind uniformly across
// every WASM handler instead of matching free-form messages.
func TaggedError(kind, msg string) js.Value {
	errVal := js.Global().Get("Error").New(msg)
	errVal.Set("kind", kind)

	return errVal
}

// StructuralError is a thin alias around TaggedError(KindStructural, ...)
// for the most common case: the bundle couldn't be parsed/validated, or
// another precondition failed and no per-path render was attempted.
// Caller treats this as "route the whole batch to cold render".
func StructuralError(msg string) js.Value {
	return TaggedError(KindStructural, msg)
}

// RenderError is a thin alias around TaggedError(KindRender, ...) for the
// other common case: the inputs/render path failed for a non-structural
// reason and the JS caller should surface or fall back accordingly.
func RenderError(msg string) js.Value {
	return TaggedError(KindRender, msg)
}
