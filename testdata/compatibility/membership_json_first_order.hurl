GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" not exists
header "X" not contains "forbidden"
GET http://example.invalid/health
HTTP 200
