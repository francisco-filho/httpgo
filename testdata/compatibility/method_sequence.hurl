POST http://example.invalid/health
{}
HTTP 200
PUT http://example.invalid/health
{"a":1}
HTTP 200
PATCH http://example.invalid/error
[1,2]
DELETE http://example.invalid/health
HTTP 200
GET http://example.invalid/missing
HTTP 404
