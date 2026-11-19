GET http://localhost:8080/health
HTTP *
[Asserts]
header "X-Test" startsWith "secret"
