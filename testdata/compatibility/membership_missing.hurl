GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.absent" not contains null
jsonpath "$.absent" not exists
GET http://example.invalid/health
HTTP 200
