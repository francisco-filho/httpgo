GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.a" == 3
jsonpath "$.b" == 3
jsonpath "$.c" == 3
jsonpath "$.items" count == 1.0
GET http://example.invalid/health
HTTP 200
