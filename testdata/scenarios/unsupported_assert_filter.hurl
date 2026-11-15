GET http://localhost:8080/health
HTTP 200
[Asserts]
header "X-Test" count == 1
