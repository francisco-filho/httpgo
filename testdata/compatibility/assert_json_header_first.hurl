GET http://example.invalid/capture
HTTP *
[Asserts]
header "X-Test" == "ok"
jsonpath "$" == null
GET http://example.invalid/health
HTTP 200
