GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" == [2,1]
GET http://example.invalid/health
HTTP 200
