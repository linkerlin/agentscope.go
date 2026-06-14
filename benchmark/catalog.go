package benchmark

// Catalog lists all benchmark functions in the project, organized by domain.
// This serves as a living index — update when adding new benchmark suites.
//
// To find benchmarks:
//
//	rg "^func Benchmark" --type go -l
var Catalog = []Category{
	{
		Name: "Gateway",
		Path: "./gateway/",
		Benchmarks: []string{
			"BenchmarkGateway_Chat",
			"BenchmarkGateway_ChatStream",
			"BenchmarkGateway_ChatConcurrent/concurrency=100",
			"BenchmarkGateway_ChatMixed",
			"BenchmarkGateway_HealthConcurrent/concurrency=100",
			"BenchmarkGateway_ChatStreamConcurrent/concurrency=50",
			"BenchmarkGateway_RealServer",
			"BenchmarkGateway_RealServerHealth",
			"BenchmarkGateway_SessionCreateConcurrent",
			"BenchmarkGateway_SessionCreateParallel",
			"BenchmarkGateway_BroadcastRoom",
		},
	},
	{
		Name: "Memory/Vector",
		Path: "./memory/vector/",
		Benchmarks: []string{
			"BenchmarkSQLiteVec_Insert",
			"BenchmarkSQLiteVec_Search",
			"BenchmarkSQLiteVec_Get",
			"BenchmarkSQLiteVec_Delete",
			"BenchmarkCosineSimilarity",
			"BenchmarkNormalizeVector",
			"BenchmarkLocalVectorStore_SearchLargeDataset/nodes=5000",
			"BenchmarkLocalVectorStore_InsertBatch/batch=100",
			"BenchmarkLocalVectorStore_GetConcurrent",
			"BenchmarkLocalVectorStore_SearchConcurrent/concurrency=50",
			"BenchmarkLocalVectorStore_MixedWorkload",
		},
	},
	{
		Name: "Plan/DAG",
		Path: "./plan/",
		Benchmarks: []string{
			"BenchmarkDAGExecutor_Sequential",
			"BenchmarkDAGExecutor_Parallel",
			"BenchmarkDAGExecutor_TopologicalSort",
			"BenchmarkDAGExecutor_WithRetry",
			"BenchmarkDAGExecutor_LargeFanOut",
			"BenchmarkValidateDAG",
			"BenchmarkReadySteps",
		},
	},
	{
		Name: "Memory/Graph",
		Path: "./memory/graph/",
		Benchmarks: []string{
			"BenchmarkFindAllPaths",
			"BenchmarkMultiHopNeighbors",
			"BenchmarkHasCycle_NoCycle",
			"BenchmarkSubgraph",
			"BenchmarkNodeImportance",
			"BenchmarkSearchNodes",
		},
	},
	{
		Name: "Tool/A2A",
		Path: "./tool/a2a/",
		Benchmarks: []string{
			"BenchmarkA2ATool_ExecuteSync",
			"BenchmarkA2ATool_ExecuteStreaming",
			"BenchmarkRegistry_AllTools",
		},
	},
}

// Category groups benchmarks by domain.
type Category struct {
	Name       string
	Path       string
	Benchmarks []string
}
