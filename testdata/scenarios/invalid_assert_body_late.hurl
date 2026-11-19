GET http://localhost:8080/health
HTTP 200
GET http://localhost:8080/export
HTTP 200
[Asserts]
body not contains "passwordHash" "extra"
