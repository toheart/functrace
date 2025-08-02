package trace

import (
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

// safeExecute is a method of TraceInstance that safely executes a function,
// recovering from any panics and logging them using the instance's logger.
func (t *TraceInstance) safeExecute(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			// Use t.log to ensure the panic is logged to the correct, configured output.
			t.log.WithFields(logrus.Fields{
				"panic": r,
				"stack": string(debug.Stack()),
			}).Error("!!! PANIC RECOVERED in functrace !!!")
		}
	}()
	fn()
}
