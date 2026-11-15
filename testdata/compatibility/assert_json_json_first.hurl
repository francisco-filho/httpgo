GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" == null
header "X-Test" == "ok"
GET http://example.invalid/health
HTTP 200
