// Package e2e hosts production-path HTTP tests for AgentScope.Go.
//
// Tests construct a gateway via NewApp + RegisterAppRoutes and drive it with
// httptest.Server (real HTTP, JWT, storage, workspace, KB, control plane).
// They complement package-level httptest suites by asserting the *assembled*
// service, which is what operators actually run.
package e2e
