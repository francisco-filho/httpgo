GET http://example.invalid/health
HTTP 200
GET http://example.invalid/capture
HTTP 200
[Captures]
baseline: jsonpath "$" count count
GET http://example.invalid/health
HTTP 200
