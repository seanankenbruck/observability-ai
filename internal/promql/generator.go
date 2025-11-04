// internal/promql/generator.go
package promql

import (
	"github.com/seanankenbruck/observability-ai/internal/processor"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

type QueryGenerator struct {
	semanticMapper *semantic.Mapper
	safety         *processor.SafetyChecker
}