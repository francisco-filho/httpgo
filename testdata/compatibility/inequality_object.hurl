GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" != {"a":1,"b":[true,null]}
GET http://example.invalid/health
