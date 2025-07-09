// internal/promql/generator.go
type QueryGenerator struct {
	semanticMapper *semantic.Mapper
	safety         *SafetyChecker
}