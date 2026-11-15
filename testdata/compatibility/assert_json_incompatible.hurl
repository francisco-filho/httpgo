GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$.price.value" == null
GET http://example.invalid/health
HTTP 200
