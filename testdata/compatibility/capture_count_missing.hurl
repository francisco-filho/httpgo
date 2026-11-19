GET http://example.invalid/capture
HTTP 200
[Captures]
baseline: jsonpath "$.items" count
GET http://example.invalid/health
HTTP 200
