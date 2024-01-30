# healthkit-test-server
Test server for testing HealthKit query handlers written using go

### Building

```bash
$ docker build --tag healthkit-test-server
$ docker run -d -p 8080:8080 healthkit-test-server
```