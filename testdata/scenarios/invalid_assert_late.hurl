GET http://localhost:8080/health
HTTP 200
GET http://localhost:8080/next
HTTP *
[Asserts]
header "Authorization" == secret
