GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.missing" == null
GET http://example.invalid/health
HTTP 200
