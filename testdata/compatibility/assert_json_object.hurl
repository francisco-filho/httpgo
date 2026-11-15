GET http://example.invalid/capture
HTTP *
[Asserts]
jsonpath "$" == {"a":1.0,"b":[true,null]}
GET http://example.invalid/health
HTTP 200
