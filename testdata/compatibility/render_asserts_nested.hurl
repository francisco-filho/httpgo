GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$" == {"n":[{{n}},true,null]}
GET http://example.invalid/health
HTTP 200
