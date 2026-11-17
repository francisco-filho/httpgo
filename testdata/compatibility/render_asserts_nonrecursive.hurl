GET http://example.invalid/capture
HTTP 200
[Captures]
saved: jsonpath "$"
GET http://example.invalid/capture
HTTP 200
[Asserts]
header "X" == "{{saved}}"
jsonpath "$" == {{saved}}
GET http://example.invalid/health
HTTP 200
