GET http://localhost:8080/first
GET http://localhost:8080/late
HTTP *
[Asserts]
jsonpath "$" >= "1"
