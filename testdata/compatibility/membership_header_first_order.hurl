GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X" not contains "forbidden"
jsonpath "$" not exists
GET http://example.invalid/health
HTTP 200
