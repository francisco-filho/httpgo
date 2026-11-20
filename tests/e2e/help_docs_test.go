package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetailedCLIReferences(t *testing.T) {
	for name, terms := range map[string][]string{
		"body":               {`body contains "customer"`, `body not contains "passwordHash"`, `body not contains "trackingToken"`, "sua negação sempre falha", "sem reserializar JSON", "Charset desconhecido", "nunca body ou fragmentos recebidos", "Captura secreta provisória", "placeholders não interpolam"},
		"headers":            {`header "Content-Type" contains "application/json"`, `header "X-Request-Id" exists`, `header "X-Request-Id" == "req-123"`, "primeira na ordem recebida", "ausente", "não passa pelo renderer", "capturas provisórias", "Set-Cookie", "Diferenças deliberadas"},
		"json":               {`jsonpath "$.name" == "Pizza"`, `jsonpath "$.price" == 4500`, `jsonpath "$.active" == true`, "sem passar por float64", "erro de avaliação JSON (2)", "captura secreta pendente na mesma resposta", "não antecipa seleção nem confirmação", `jsonpath "$.items" count == 3`, `jsonpath "$.total" >= 1`, `jsonpath "$.items[0].id" exists`, `jsonpath "$.name" matches "^Pizza"`, "count em strings/objetos", "Count em captura JSONPath é suportado", "arbitrários não marcados"},
		"external files":     {"file,<caminho>;", "file,../payloads/product.json;", "diretório do cenário", "Não há sandbox", "sem escapes nem interpolação", "Sem --render-files", "extensão HTTPGo", "Arquivos multipart permanecem literais", "causa bruta, conteúdo", "binário", "código 2"},
		"config":             {".httpgo.yaml", "diretório corrente", "0.001 e 86400", "1 MiB", "version: 1", "extensão HTTPGo", "${NOME}"},
		"environments":       {"--env=staging", "no máximo um ambiente", "Repetição, mesmo com nome idêntico", "environments/<nome>.yaml", "inclusive se vazia", "defaults internos < YAML base < ambiente selecionado < CLI", "escalares, listas e null substituem", "lista vazia posterior"},
		"process references": {"${OWNER_TOKEN}", "[A-Za-z_][A-Za-z0-9_]*", "ocupando todo o valor string", "base < ambiente < CLI", "referências ainda efetivas", "inclusive sem uso no cenário", "Vazio é string vazia", "marcado antes dos overrides", "credenciais reais em argumentos seguem proibidas", "Não importa variáveis não referenciadas", "sem cache global, carga automática de .env", "Não há stdin"},
		"suites":             {"um ou mais caminhos de arquivos regulares legíveis e diretórios", "--fail-fast", "não executados", "resumo", "unidades", "cookie jar"},
	} {
		t.Run(name, func(t *testing.T) { assertDocumentContains(t, "docs/subset.md", terms...) })
	}
}

// Detailed language contracts live in docs, not in the quick CLI reference.
// Ignore Markdown emphasis/code delimiters and wrapping, not the contract text.
func assertDocumentContains(t *testing.T, path string, terms ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../..", path))
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(text string) string {
		text = strings.ReplaceAll(text, "`", "")
		text = strings.ReplaceAll(text, "**", "")
		return strings.Join(strings.Fields(text), " ")
	}
	document := normalize(string(data))
	for _, term := range terms {
		if !strings.Contains(document, normalize(term)) {
			t.Errorf("%s missing %q", path, term)
		}
	}
}
