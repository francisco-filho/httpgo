GET http://example.invalid/capture
HTTP 200
[Captures]
x: regex "(42)"
y: jsonpath "$"
z: header "X-Test" regex "v=(.+)"
POST http://example.invalid/health
Content-Type: application/json
{"regex":{{x}},"json":{{y}},"header":"{{z}}"}
HTTP 200
