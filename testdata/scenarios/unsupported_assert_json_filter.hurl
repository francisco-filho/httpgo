GET http://localhost:8080/late
HTTP *
[Asserts]
jsonpath "$" length == 1
