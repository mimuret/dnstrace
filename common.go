// SPDX-License-Identifier: Apache-2.0
package dnstrace

import (
	"go.opentelemetry.io/otel/trace"
)

func newTracer(tp trace.TracerProvider) trace.Tracer {
	return tp.Tracer("github.com/mimuret/dnstrace")
}
