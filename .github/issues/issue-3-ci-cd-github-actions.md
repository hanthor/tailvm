# Issue: Configure GitHub Actions CI/CD Pipeline

## 📌 Goal
Establish a continuous integration pipeline using GitHub Actions to automatically run linters, compile binaries for multiple architectures (`amd64`, `arm64`), run unit tests, and trigger live integration testing inside a mock cluster.

## 🛠️ Requirements

### 1. Code Quality & Compilation Pipeline (`.github/workflows/ci.yml`)
- **Go Setup**: Initialize Go 1.26 setup.
- **Linter**: Run `golangci-lint` to check for formatting errors, unused code, and static analysis warnings.
- **Compilation Check**: Assert that the code builds successfully on Linux (`amd64` and `arm64`) using:
  ```bash
  GOOS=linux GOARCH=amd64 go build -v -o tailvm-linux-amd64 .
  GOOS=linux GOARCH=arm64 go build -v -o tailvm-linux-arm64 .
  ```
- **Unit Test Runner**: Execute the complete unit test suite on every pull request or push:
  ```bash
  go test -race -coverprofile=coverage.txt -covermode=atomic ./...
  ```
- **Coverage Publishing**: Publish coverage reports to Codecov or directly in the pull request comments.

### 2. Integration Pipeline on Virtualized Runner
- **K3s / Minikube in Actions**: Use the `k8s-actions/setup-k3s` or `minikube` GitHub Action to spawn a lightweight, real-world Kubernetes cluster on the runner.
- **Bootstrapping operator test**: Run the `./tailvm bootstrap` command inside the workflow to install KubeVirt and CDI on the K3s cluster.
- **Integration Test Execution**: Trigger the integration testing harness:
  ```bash
  go test -tags=integration -v ./...
  ```

## 📈 Success Criteria
- Pull Requests automatically execute the complete lint, compilation, unit test, and integration suites.
- Failed tests block merging into the `main` branch, ensuring stable main-branch builds.

---

## 📋 Sub-Issues & Granular Tasks Checklist

- [ ] **Sub-Issue 3.1: Static Quality Verification Pipeline**
  - [ ] Configure `golangci-lint` workflow step inside GitHub Actions.
  - [ ] Set up Go compilation validation block for multi-arch compilation (amd64/arm64).
  - [ ] Configure race-detector unit test script `go test -race ./...`.
- [ ] **Sub-Issue 3.2: Automated Coverage Tracking**
  - [ ] Configure `-coverprofile` outputs and hook them to Codecov or PR comments.
- [ ] **Sub-Issue 3.3: Virtualized Cluster Setup inside Workflows**
  - [ ] Configure K3d or Minikube action on the GitHub runner container.
  - [ ] Implement a workflow script that applies KubeVirt and CDI manifests onto the runner cluster.
  - [ ] Trigger the live integration test suite against the runner cluster and assert full success.

