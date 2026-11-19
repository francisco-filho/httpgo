GET http://example.invalid/capture
HTTP 200
[Captures]
baseline: jsonpath "$" count
[Asserts]
jsonpath "$" count == {{baseline}}
GET http://example.invalid/health
HTTP 200
