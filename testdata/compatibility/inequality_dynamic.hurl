GET http://example.invalid/capture
HTTP 200
[Captures]
old: jsonpath "$.token"
GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.token" != "{{old}}"
GET http://example.invalid/health
