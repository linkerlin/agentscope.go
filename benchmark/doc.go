// Package benchmark provides a centralized benchmark suite for AgentScope.Go.
//
// It catalogs all benchmark functions across the project, organized by domain,
// and provides documentation on how to run and compare benchmarks.
//
// # Running Benchmarks
//
//	# Run all benchmarks
//	make bench
//
//	# Save baseline
//	make bench-save
//
//	# Compare against baseline
//	make bench-compare
//
//	# Profile a specific package
//	make bench-cpu PKG=./memory/vector
//	make bench-mem PKG=./plan
//
// # Benchmark Categories
//
//   - Gateway:       ./gateway/ (HTTP/SSE throughput + concurrency)
//   - Memory:        ./memory/vector/ (Insert/Search/Get/Delete + mixed workload)
//   - Plan/DAG:      ./plan/ (topological sort + parallel execution)
//   - Graph:         ./memory/graph/ (path finding + cycle detection + search)
//   - Tool/A2A:      ./tool/a2a/ (distributed ReAct tool execution)
//   - Agent:         ./agent/ (ReAct loop + structured output)
//   - Model:         ./model/ (formatter + router)
//
// # Metrics
//
// All benchmarks report:
//   - ns/op: nanoseconds per operation
//   - B/op: bytes allocated per operation
//   - allocs/op: number of allocations per operation
//
// Use benchstat for statistical comparison between runs.
package benchmark
