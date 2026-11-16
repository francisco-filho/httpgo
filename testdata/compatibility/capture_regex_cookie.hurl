GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "sEt-CoOkIe" regex "refresh=([^;]+)"
GET http://example.invalid/health
Cookie: refresh={{x}}
HTTP 200
