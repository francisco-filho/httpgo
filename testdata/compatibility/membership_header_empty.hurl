GET http://example.invalid/capture
HTTP 200
[Asserts]
header "x" not contains "Domain"
GET http://example.invalid/health
HTTP 200
