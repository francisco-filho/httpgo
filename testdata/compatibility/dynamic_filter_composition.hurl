GET http://example.invalid/capture
HTTP 200
[Asserts]
jsonpath "$[?(@.v=='Order {{suffix}}')].id" == 1

GET http://example.invalid/health
HTTP 200
