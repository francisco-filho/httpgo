GET http://example.invalid/poll
HTTP 200
[Captures]
baseline: jsonpath "$.items" count
GET http://example.invalid/poll
HTTP 200
[Asserts]
jsonpath "$.items" count == {{baseline}}
GET http://example.invalid/health
HTTP 200
