GET http://localhost:8080/health
HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
header "X-Request-Id" exists
header "X-Request-Id" == "req-123"
