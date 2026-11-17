POST http://example.invalid/poll
HTTP 200
[Captures]
old: jsonpath "$.token"
POST http://example.invalid/poll
HTTP 200
[Asserts]
jsonpath "$.token" != "{{old}}"
GET http://example.invalid/health
