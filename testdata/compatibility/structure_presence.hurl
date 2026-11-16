GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" exists
jsonpath "$.id" exists
jsonpath "$.s" exists
jsonpath "$.a" exists
jsonpath "$.o" exists
jsonpath "$.b" exists
jsonpath "$.n" exists
GET http://example.invalid/health
HTTP 200
