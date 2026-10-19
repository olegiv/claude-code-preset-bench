# Team notes

- Two developers, both writing Go. No on-call for infrastructure: the platform team operates its data services.
- linkbox uses only the Go standard library. A third-party module needs a security review by the platform team
  first, which takes about a week; well-known, maintained modules pass it.
- CI runs `go test ./...` in a container that cannot reach the platform's services, but it can start service
  containers, a database for example, next to the tests.
