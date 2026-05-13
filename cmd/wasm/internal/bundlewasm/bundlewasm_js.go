//go:build js && wasm

package bundlewasm

import "syscall/js"

// StructuralError builds the JS Error object for the structural path: the
// bundle couldn't be parsed/validated, or another precondition failed and
// no per-path render was attempted. Caller treats this as "route the
// whole batch to cold render".
func StructuralError(msg string) js.Value {
	errVal := js.Global().Get("Error").New(msg)
	errVal.Set("kind", KindStructural)

	return errVal
}
