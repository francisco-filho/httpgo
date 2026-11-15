GET http://example.invalid/health
HTTP 200
[Asserts]
header "content-type" contains "application/json"
header "X-Request-Id" exists
header "x-request-id" == "req-123"
GET http://example.invalid/missing
HTTP 404
