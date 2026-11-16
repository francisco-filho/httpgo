GET http://localhost:8080/first
GET http://localhost:8080/late
HTTP 200
[Captures]
size: jsonpath "$.items" count
