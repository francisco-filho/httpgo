GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$.a" == []
jsonpath "$.o" == {}
GET http://example.invalid/health
HTTP 200
