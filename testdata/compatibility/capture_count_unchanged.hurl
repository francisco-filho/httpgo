GET http://example.invalid/poll
HTTP 200
[Captures]
baseline: jsonpath "$.items" count
POST http://example.invalid/health
Content-Type: application/json
{"productId":"p1"}
HTTP 200
GET http://example.invalid/poll
HTTP 200
[Asserts]
jsonpath "$.items" count == {{baseline}}
jsonpath "$.items" count >= {{baseline}}
jsonpath "$.items" count <= {{baseline}}
GET http://example.invalid/health
HTTP 200
