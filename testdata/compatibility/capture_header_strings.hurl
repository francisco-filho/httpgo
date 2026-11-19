GET http://example.invalid/capture
HTTP 200
[Captures]
x: header "X-Number"
y: header "X-Boolean"
z: header "X-JSON"
POST http://example.invalid/health
Content-Type: application/json
ETag: {{x}}
X-Probe: {{y}}
{"number":{{x}},"boolean":{{y}},"json":{{z}}}
HTTP 200
